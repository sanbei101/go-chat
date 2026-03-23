-- name: UserExistsByEmail :one
SELECT EXISTS (
    SELECT 1
    FROM users
    WHERE email = @email
);

-- name: CreateUser :one
INSERT INTO users (
    username,
    email,
    password
) VALUES (
    @username,
    @email,
    @password
)
RETURNING id, username, email, password, create_date;

-- name: GetUserByEmail :one
SELECT id, username, email, password, create_date
FROM users
WHERE email = @email
LIMIT 1;
