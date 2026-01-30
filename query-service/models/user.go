package models

type Product struct {
	ID          string  `bson:"_id,omitempty" json:"id"`
	Description string  `bson:"p_desc" json:"p_desc"`
	Quantity    float64 `bson:"qte" json:"qte"`
}
