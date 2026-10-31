CREATE TABLE users (
    id            UUID PRIMARY KEY,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    -- UNIQUE é a garantia real contra e-mails duplicados, mesmo com requisições simultâneas
    email         TEXT NOT NULL UNIQUE,
    -- guarda o hash bcrypt, nunca a senha em texto
    password_hash TEXT NOT NULL
);
