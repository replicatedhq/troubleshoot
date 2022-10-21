package types

type Layers interface {
	GetAllowedMediaTypes() []string
	GetSpec() []byte
	GetValues() []byte
	SetLayer(medaiType string, data []byte)
	IsEmpty() bool
}
