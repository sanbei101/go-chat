DROP TABLE IF EXISTS "room_message";
DROP TABLE IF EXISTS "room_member";
DROP TABLE IF EXISTS "room";
DROP TABLE IF EXISTS "users";

-- 用户表
CREATE TABLE "users" (
    "id" bigserial PRIMARY KEY,
    "username" varchar NOT NULL,
    "email" varchar NOT NULL UNIQUE,
    "password" varchar NOT NULL,
    "create_date" timestamp NOT NULL DEFAULT now()
);

-- 聊天室表
CREATE TABLE "room" (
    "id" bigserial PRIMARY KEY,
    "name" varchar NOT NULL,
    "create_date" timestamp NOT NULL DEFAULT now()
);

-- 聊天室成员表
CREATE TABLE "room_member" (
    "id" bigserial PRIMARY KEY,
    "room_id" bigint NOT NULL,
    "user_id" bigint NOT NULL,
    "join_date" timestamp NOT NULL DEFAULT now(),
    "last_online" timestamp NOT NULL DEFAULT now(),
    FOREIGN KEY ("room_id") REFERENCES "room" ("id"),
    FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

-- 聊天室消息表
CREATE TABLE "room_message" (
    "id" bigserial PRIMARY KEY,
    "room_id" bigint NOT NULL,
    "user_id" bigint NOT NULL,
    "message" text NOT NULL,
    "created_at" timestamp NOT NULL DEFAULT now(),
    FOREIGN KEY ("room_id") REFERENCES "room" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
);