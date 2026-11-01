DELETE FROM users
WHERE id IN (
    SELECT id
    FROM (
        SELECT id, row_number() OVER (PARTITION BY lower(email) ORDER BY email = lower(email) DESC, id) AS rank
        FROM users
    ) ranked
    WHERE rank > 1
);

UPDATE users SET email = lower(email) WHERE email <> lower(email);

ALTER TABLE users ADD CONSTRAINT users_email_lowercase CHECK (email = lower(email));
