package importer

import "encoding/json"

// Message — задача импорта в очереди q.import. api_key передаётся здесь (в БД не хранится).
type Message struct {
	JobID     string `json:"job_id"`
	Source    string `json:"source"`
	Region    string `json:"region"`
	Subdomain string `json:"subdomain"`
	Mode      string `json:"mode"`
	APIKey    string `json:"api_key"`
}

// Marshal сериализует сообщение задачи импорта.
func (m Message) Marshal() ([]byte, error) { return json.Marshal(m) }

// ParseMessage разбирает сообщение задачи импорта.
func ParseMessage(b []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(b, &m)
	return m, err
}
