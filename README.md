# Esmaga Link

[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Cassandra](https://img.shields.io/badge/Cassandra-1287B1?style=for-the-badge&logo=apache-cassandra&logoColor=white)](https://cassandra.apache.org/)
[![Redis](https://img.shields.io/badge/Redis-DC382D?style=for-the-badge&logo=redis&logoColor=white)](https://redis.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![RabbitMQ](https://img.shields.io/badge/RabbitMQ-FF6600?style=for-the-badge&logo=rabbitmq&logoColor=white)](https://www.rabbitmq.com/)
[![Flutter](https://img.shields.io/badge/Flutter-02569B?style=for-the-badge&logo=flutter&logoColor=white)](https://flutter.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-003B57?style=for-the-badge&logo=sqlite&logoColor=white)](https://www.sqlite.org/)
[![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?style=for-the-badge&logo=javascript&logoColor=black)](https://developer.mozilla.org/pt-BR/docs/Web/JavaScript)

Um ecossistema de encurtamento de URLs projetado para ser resiliente a falhas, processar milhares de requisições por segundo e equilibrar simplicidade de uso anônimo com análises avançadas para contas premium.

## Visão Geral

O **Esmaga Link** oferece uma experiência de encurtamento de URLs com foco em escalabilidade extrema e privacidade. Usuários anônimos podem criar links imutáveis de forma rápida, enquanto contas autenticadas desfrutam de edição dinâmica, estatísticas detalhadas e gerenciamento completo.

## Principais Tecnologias

| Camada            | Tecnologias                                                       |
| ----------------- | ----------------------------------------------------------------- |
| **Backend API**   | Golang (goroutines, channels, binário estático)                   |
| **Frontend Web**  | JavaScript/TypeScript (SSR + SPA)                                 |
| **App Mobile**    | Flutter (offline-first, SQLite)                                   |
| **Armazenamento** | PostgreSQL (contas), Apache Cassandra (cliques), Redis (cache/ID) |
| **Mensageria**    | RabbitMQ (processamento assíncrono)                               |

## Documentação Completa

Todos os requisitos funcionais, não funcionais, regras de negócio, detalhes de arquitetura e roadmap estão documentados no arquivo:

[**Documento de Requisitos**](./docs/requisitos.md)

## Roadmap

| Fase                                | Status     |
| ----------------------------------- | ---------- |
| Requisitos                          | Concluído  |
| Armazenamento (modelagem inicial)   | Pendente   |
| Motor de geração de IDs             | Pendente   |
| Redirecionamento + filas            | Pendente   |
| Workers assíncronos                 | Pendente   |
| Frontend Web (landing + dashboard)  | Pendente   |
| App Mobile (Flutter)                | Pendente   |
| Segurança (rate limiting, validação)| Pendente   |

*Para detalhes completos, consulte o [documento de requisitos](./docs/requirements.md).*
