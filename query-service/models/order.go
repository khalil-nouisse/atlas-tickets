package models

import "time"

type Order struct {
	ID           string    `bson:"_id,omitempty" json:"id_commande"`
	ClientID     string    `bson:"id_client" json:"id_client"`
	DateCommande time.Time `bson:"date_commande" json:"date_commande"`
}
