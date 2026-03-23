DROP TABLE IF EXISTS "users";

CREATE TABLE "users" (
    "id" bigserial PRIMARY KEY,
    "username" varchar NOT NULL,
    "email" varchar NOT NULL UNIQUE,
    "password" varchar NOT NULL,
    "create_date" timestamp NOT NULL DEFAULT now()
)

DROP TABLE IF EXISTS "room";

CREATE TABLE "room" (
    "id" bigserial PRIMARY KEY,
    "name" varchar NOT NULL,
    "create_date" timestamp NOT NULL DEFAULT now()
)

DROP TABLE IF EXISTS "room_member";

CREATE TABLE "room_member" (
    "id" bigserial PRIMARY KEY,
    "room_id" bigint NOT NULL,
    "user_id" bigint NOT NULL,
    "join_date" timestamp NOT NULL DEFAULT now(),
    "last_online" timestamp NOT NULL DEFAULT now(),
    FOREIGN KEY ("room_id") REFERENCES "room" ("id"),
    FOREIGN KEY ("user_id") REFERENCES "users" ("id")
)

DROP TABLE IF EXISTS room_message;

CREATE TABLE "room_message" (
    "id" bigserial PRIMARY KEY,
    "room_id" bigint NOT NULL,
    "user_id" bigint NOT NULL,
    "message" text NOT NULL,
    "created_at" timestamp NOT NULL DEFAULT now(),
    FOREIGN KEY ("room_id") REFERENCES "room" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
); 
