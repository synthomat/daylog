CREATE TABLE IF NOT EXISTS attachments
(
    id         uuid
        constraint attachments_pk
            primary key,
    post_id    uuid
        constraint attachments_posts_id_fk
            references posts,
    created_at datetime default CURRENT_TIMESTAMP not null,
    in_use     boolean  default false,
    file_path  text                               not null
);

