# Esmaga Link

Um ecossistema de encurtamento de URLs projetado para ser resiliente a falhas, processar milhares de requisições por segundo e equilibrar a simplicidade do uso anônimo com análises avançadas para contas premium.

## Visão Geral

O Esmaga Link oferece uma experiência de encurtamento de URLs com foco em escalabilidade extrema e privacidade. Usuários anônimos podem criar links imutáveis de forma rápida, enquanto contas autenticadas desfrutam de edição dinâmica, estatísticas detalhadas e gerenciamento completo.

## Principais Tecnologias

  * **Load Balancer**: Nginx (terminação TLS e distribuição de carga).
  * **Backend API**: Golang (múltiplas instâncias em cluster, goroutines, channels).
  * **Armazenamento**: PostgreSQL (contas), Apache Cassandra (cliques), Redis Duplo (cache e coordenação de IDs).
  * **Mensageria**: RabbitMQ (processamento assíncrono de eventos).
  * **App Mobile**: Flutter (estratégia offline-first com SQLite).
  * **Frontend Web**: JavaScript/TypeScript (abordagem híbrida SSR + SPA).

## Diferenciais Técnicos e Arquitetura

Para atingir as metas de performance e segurança, o sistema utiliza soluções específicas de engenharia:

  * **Escalabilidade Horizontal (Stateless)**: O backend foi desenhado para rodar em múltiplas instâncias simultâneas, permitindo que o sistema cresça conforme a demanda.
  * **Motor de Geração de IDs**: Implementa reserva de IDs em lote via Redis para eliminar latência de rede e evitar colisões entre instâncias diferentes.
  * **Alta Disponibilidade (HA)**: O Load Balancer monitora a saúde das instâncias e redireciona o tráfego automaticamente em caso de falha de um nó.
  * **Ofuscação e Eras**: Os identificadores são processados com Base62 customizada e multiplicadores primos para garantir que links sequenciais não sejam previsíveis.
  * **Performance (SLO)**: O sistema é desenhado para resolver 99% das requisições de redirecionamento em menos de 15ms.

## Roadmap de Desenvolvimento

| Fase | Módulo       | Descrição                                        | Status    |
| :--- | :----------- | :----------------------------------------------- | :-------- |
| 0    | Requisitos   | Documentação de objetivos e arquitetura          | Concluído |
| 1    | Infra e Auth | Setup de DBs, Load Balancer e Autenticação JWT   | Pendente  |
| 2    | Core: ID Gen | Motor de IDs (Redis e Base62 Customizada)        | Pendente  |
| 3    | Segurança    | Rate Limiting e integração com Safe Browsing     | Pendente  |
| 4    | API: Escrita | Endpoints de criação e gestão de links           | Pendente  |
| 5    | API: Leitura | Redirecionamento de alta performance (15ms)      | Pendente  |
| 6    | Analytics    | Workers para processamento assíncrono de cliques | Pendente  |
| 7    | Web Front    | Dashboard Premium (SPA) e Landing Page (SSR)     | Pendente  |
| 8    | Mobile       | Aplicativo Flutter base com persistência SQLite  | Pendente  |
| 9    | Sync         | Lógica de sincronização offline para o App       | Pendente  |

## Documentação

Para detalhes completos sobre regras de negócio, esquemas de dados e especificações técnicas, consulte o arquivo disponível em:

[Documento de Requisitos](./docs/requirements.md)
