CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE client(
    id_client UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    firstname VARCHAR(15) NOT NULL,
    lastname VARCHAR(15) NOT NULL,
    address VARCHAR(100)
);

CREATE TABLE product(
    id_product UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    p_description VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    stock_quantity INT NOT NULL DEFAULT 0
);


CREATE TABLE commande(
    id_commande UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_client UUID NOT NULL REFERENCES client(id_client) ON DELETE CASCADE,
    date_commande TIMESTAMP DEFAULT CURRENT_TIMESTAMP

);


CREATE TABLE commande_prod(
    id_commande_prod UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    id_commande UUID NOT NULL REFERENCES commande(id_commande) ON DELETE CASCADE,
    id_produit UUID NOT NULL REFERENCES product(id_product) ON DELETE CASCADE,
    prix_unitaire DECIMAL(10,2) NOT NULL , 
    quantity INT NOT NULL
);