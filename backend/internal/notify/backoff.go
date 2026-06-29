package notify

import "time"

// retrySchedule — задержки повторной доставки с возрастающим backoff (DESIGN §8.1: «1м → 5м → 30м»).
// Индекс = номер попытки (attempt) минус 1. После исчерпания списка ретраи прекращаются.
var retrySchedule = [...]time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
}

// MaxAttempts — сколько раз сообщение доставляется повторно, прежде чем уйти в DLQ.
const MaxAttempts = len(retrySchedule)

// RetryBackoff возвращает задержку перед попыткой номер attempt (1-based) и признак того, что
// повторная доставка ещё допустима. attempt<=0 трактуется как первая (без задержки нет смысла).
// Когда attempt превышает длину расписания — (0, false): ретраи исчерпаны, сообщение → DLQ.
func RetryBackoff(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(retrySchedule) {
		return 0, false
	}
	return retrySchedule[attempt-1], true
}
