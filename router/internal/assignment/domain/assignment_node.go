package assignment

type AssignmentNode struct {
	// ID — Уникальный идентификатор исполнителя; должен совпадать с его аргументом --node.
	ID string `json:"id"`
	// URL — HTTP origin исполнителя без пути и параметров; адрес должен быть доступен из Router.
	URL string `json:"url"`
}
