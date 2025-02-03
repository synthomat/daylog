CREATE TABLE IF NOT EXISTS "posts"
(
    id         text
        primary key,
    created_at datetime not null,
    updated_at datetime,
    deleted_at datetime,
    event_time datetime not null,
    title      text,
    body       text not null
)