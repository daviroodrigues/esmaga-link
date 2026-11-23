# Documento de Requisitos – Esmaga Link

## Visão Geral do Produto

O **Esmaga Link** será um ecossistema de encurtamento de URLs projetado para ser resiliente a falhas e capaz de processar milhares de requisições por segundo através de uma arquitetura distribuída.

## Regras de Negócio e Comportamento

### Perfis de Uso

  * **Links Anônimos:**
      * Imutabilidade Estrita: Não podem ser alterados após a criação.
      * Privacidade: Apenas métricas agregadas.
  * **Links Autenticados:**
      * Edição Dinâmica: Permite alterar o destino (invalida cache).
      * Analytics de Precisão: Dados detalhados (geografia, dispositivo, OS).
      * Persistência Pós-Exclusão: Tornam-se anônimos e imutáveis se a conta for deletada.

## Arquitetura de Sistema (System Design)

### Topologia de Rede e Balanceamento

Para garantir que o sistema nunca fique offline e suporte picos de tráfego, a entrada de requisições será gerenciada por uma camada de balanceamento:

1.  **Terminação TLS**: O Load Balancer (Nginx/HAProxy) gerencia os certificados SSL, aliviando o processamento das instâncias Go.
2.  **Health Checks**: O balanceador monitora constantemente o status das instâncias. Se uma instância apresentar falha crítica, ela é removida do pool automaticamente.
3.  **Distribuição Stateless**: Como as instâncias não guardam estado local, qualquer requisição pode ser atendida por qualquer nó disponível.

### O Motor de Geração de IDs (Escalabilidade Horizontal)

A lógica de **Reserva em Lote** via Redis é o coração que permite múltiplas instâncias trabalharem juntas sem conflito:

1.  Cada instância da API solicita um "bloco" de números ao Redis (ex: Instância A pega 1.000-2.000, Instância B pega 2.001-3.000).
2.  Os IDs são consumidos localmente pela instância.
3.  Isso elimina a necessidade de travar o banco de dados a cada novo link criado, garantindo performance linear.

### O Conceito de "Eras" e Ofuscação

* **Base62 Customizada:** Alfabeto embaralhado via Seed em variáveis de ambiente.
* **Multiplicador Coprimo Otimizado (Fibonacci Hashing):** Garante que IDs sequenciais resultem em hashes visualmente caóticos e distantes entre si.

$$\text{ID}_{\text{final}} = (\text{ID}_{\text{sequencial}} \times \text{Primo}_{\text{da Era}}) \bmod \text{Tamanho}_{\text{da Era}} + \text{Início}_{\text{da Era}}$$

* **Por que o multiplicador precisa ser coprimo?**
Para que a transformação acima gere uma permutação completa (bijetora) dos valores dentro da Era, ou seja, sem colisões e cobrindo todo o intervalo, é estritamente necessário que o multiplicador e o tamanho do intervalo sejam coprimos, representados pela regra $\mathrm{mdc}(\text{Primo}_{\text{da Era}}, \text{Tamanho}_{\text{da Era}}) = 1$.
  * Sendo coprimos, o multiplicador possui um inverso multiplicativo módulo o tamanho do intervalo. Isso assegura que cada ID sequencial gere um valor único e que absolutamente todos os valores do intervalo sejam utilizados de forma embaralhada.
  * Caso contrário, a função perde a sua injetividade, causando colisões diretas e um imenso desperdício de espaço no intervalo, o que inviabilizaria por completo a geração de identificadores únicos para o encurtador.

* **A Regra de Ouro (Proporção Áurea):**
Para evitar padrões de "tabuada" (primos muito pequenos) ou de "contagem regressiva" (primos muito próximos ao teto do intervalo), o multiplicador ideal é calculado utilizando a Proporção Áurea ($\approx 0.618033$). O sistema deve calcular $\text{Tamanho}_{\text{da Era}} \times 0.618033$ e selecionar o número coprimo mais próximo a esse alvo. Isso garante que os saltos entre os IDs consecutivos oscilem agressivamente de uma ponta a outra do espectro, eliminando qualquer previsibilidade visual para os usuários.

## Estratégia de Dados e Armazenamento

| Tecnologia           | Papel Principal       | Justificativa                                            |
| :------------------- | :-------------------- | :------------------------------------------------------- |
| **Load Balancer**    | Distribuição          | Garante Alta Disponibilidade (HA) e Redundância.         |
| **PostgreSQL**       | Backoffice e Usuários | Integridade referencial para contas.                     |
| **Apache Cassandra** | Core de Links e Logs  | Escrita ultrarrápida para milhares de eventos de clique. |
| **Redis (ID)**       | Coordenador           | Contador atômico para distribuição de lotes.             |
| **Redis (Cache)**    | Cache                 | SLO de 15ms no redirecionamento (Política LRU).          |

## Processamento Assíncrono (Event-Driven)

O redirecionamento (302) é prioridade absoluta. O registro de métricas ocorre via **RabbitMQ**:
1. Usuário clica -> API envia 302 imediato.
2. API publica "evento de clique" na fila.
3. Workers processam a fila e alimentam o Cassandra em segundo plano.

## Metas de Performance (SLO)

  * **Latência de Redirecionamento:** 99% das requisições em menos de **15ms**.
  * **Disponibilidade**: Design focado em "Zero Downtime" durante deploys.

## Backend API (Go)

* **Alta Concorrência:** Uso de *Goroutines* para gerenciar o I/O de rede e *Channels* para comunicação interna.
* **Múltiplas Instâncias:** O binário estático permite subir dezenas de instâncias idênticas em segundos.

## Roadmap de Implementação

| Status | Fase / Módulo            | Entrega Principal    | Descrição                                                                    |
| :----: | :----------------------- | :------------------- | :--------------------------------------------------------------------------- |
| `[X]`  | **Fase 0: Requisitos**   | Documentação         | Definição de objetivos e arquitetura.                                        |
| `[ ]`  | **Fase 1: Infra & Auth** | Setup DBs + LB + JWT | Configuração de Postgres/Cassandra/Redis, Load Balancer e sistema de tokens. |
| `[ ]`  | **Fase 2: Core: ID Gen** | Motor de IDs         | Implementação da reserva em lote no Redis e lógica de ofuscação.             |
| `[ ]`  | **Fase 3: Segurança**    | Proteção de Borda    | Rate Limiting e integração com Google Safe Browsing.                         |
| `[ ]`  | **Fase 4: API: Escrita** | Endpoints de Criação | Lógica em Go para salvar links e gerenciar permissões.                       |
| `[ ]`  | **Fase 5: API: Leitura** | Redirecionamento     | Foco no SLO de 15ms.                                                         |





