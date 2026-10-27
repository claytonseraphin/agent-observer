-- +goose Up
create table projects (
    id         text primary key,
    name       text not null,
    key_hash   text unique,
    key_hint   text,
    created_at timestamptz not null default now()
);

create unique index projects_name_lower_idx on projects (lower(name));

create table project_agents (
    project_id   text not null references projects(id) on delete cascade,
    name         text not null,
    upstream_url text not null,
    created_at   timestamptz not null default now(),
    primary key (project_id, name)
);

-- +goose Down
drop table project_agents;
drop table projects;
