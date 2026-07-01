// Command worker-import — асинхронная миграция данных из внешних сервисов (этап 7.5).
// Потребляет q.import (ручной ack), для каждой задачи тянет данные источника через адаптер
// (MVP: StatusPal) и пишет их в модель страницы идемпотентно (external_id_map), не рассылая
// уведомления по историческим данным. Итог фиксируется в import_jobs (status + report).
//
// Требует DATABASE_URL и RABBITMQ_URL. Реальный прогон импорта — на прод-деплое
// ([ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА]: сверка StatusPal API, согласие подписчиков 152-ФЗ).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/importer"
	"github.com/healthpage/backend/internal/queue"
	"github.com/healthpage/backend/internal/store"
)

const prefetch = 4

func main() {
	cfg := config.Load()

	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()

	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	conn, err := queue.Dial(cfg.MustRabbitMQURL())
	if err != nil {
		log.Fatalf("queue dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	setupCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("queue channel: %v", err)
	}
	if err := queue.DeclareTopology(setupCh); err != nil {
		log.Fatalf("declare topology: %v", err)
	}
	_ = setupCh.Close()

	engine := importer.NewEngine(st, importer.NewStatusPal())

	ch, err := conn.Consume(queue.QueueImport, prefetch, func(d queue.Delivery) {
		processImport(st, engine, d)
	})
	if err != nil {
		log.Fatalf("consume: %v", err)
	}
	defer func() { _ = ch.Close() }()

	log.Println("worker-import: consuming q.import")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("worker-import: stopping")
}

// processImport выполняет одну задачу импорта. Исход фиксируется в import_jobs; сообщение всегда
// Ack (провал — это состояние задачи, а не повод для бесконечных ретраев очереди).
func processImport(st *store.Store, engine *importer.Engine, d queue.Delivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	msg, err := importer.ParseMessage(d.Body)
	if err != nil {
		log.Printf("worker-import: bad message: %v", err)
		_ = d.Ack(false)
		return
	}
	jobID, err := uuid.Parse(msg.JobID)
	if err != nil {
		_ = d.Ack(false)
		return
	}
	job, err := st.ImportJobByID(ctx, jobID)
	if err != nil {
		log.Printf("worker-import: job %s not found: %v", msg.JobID, err)
		_ = d.Ack(false)
		return
	}

	if _, err := st.UpdateImportJob(ctx, job.ID, domain.ImportRunning, job.Report, "", nil); err != nil {
		log.Printf("worker-import: mark running %s: %v", job.ID, err)
		_ = d.Nack(false, true) // транзиентная ошибка БД — вернуть в очередь
		return
	}

	creds := domain.ImportCreds{APIKey: msg.APIKey, Region: domain.ImportRegion(msg.Region), Subdomain: msg.Subdomain}
	report, runErr := engine.Run(ctx, job, creds)
	now := time.Now().UTC()
	status := domain.ImportCompleted
	errMsg := ""
	if runErr != nil {
		status = domain.ImportFailed
		errMsg = runErr.Error()
	}
	if _, err := st.UpdateImportJob(ctx, job.ID, status, report, errMsg, &now); err != nil {
		log.Printf("worker-import: finalize %s: %v", job.ID, err)
	}
	log.Printf("worker-import: job %s → %s (created c=%d i=%d m=%d s=%d)",
		job.ID, status, report.ComponentsCreated, report.IncidentsCreated, report.MaintenancesCreated, report.SubscribersImported)
	_ = d.Ack(false)
}
