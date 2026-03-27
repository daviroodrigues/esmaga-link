# Documento de Requisitos – Esmaga Link

## Visão Geral do Produto

O **Esmaga Link** será um ecossistema de encurtamento de URLs projetado para ser resiliente a falhas e capaz de processar milhares de requisições por segundo. Ele deverá equilibrar a simplicidade do uso anônimo com a profundidade analítica de contas premium.

## Regras de Negócio e Comportamento

### Perfis de Uso

O sistema diferenciará estritamente como os dados serão tratados dependendo da origem da requisição:

  * **Links Anônimos:**

      * Deverão ser criados sem necessidade de login.
      * **Imutabilidade Estrita:** Uma vez gerada, a URL de destino jamais poderá ser alterada.
      * **Visibilidade:** Não haverá interface de gerenciamento para o criador.
      * **Privacidade:** Coletará apenas métricas agregadas para fins operacionais internos.

  * **Links Autenticados:**

      * Estarão atrelados a uma identidade via JWT.
      * **Edição Dinâmica:** O dono poderá alterar o destino da URL a qualquer momento (o que disparará a invalidação de caches).
      * **Analytics de Precisão:** Fornecerá acesso a dados detalhados como origem geográfica, tipo de dispositivo e sistema operacional.
      * **Persistência Pós-Exclusão:** Se um usuário deletar sua conta, seus links permanecerão ativos para evitar "links quebrados" na web, mas perderão o vínculo de propriedade e se tornarão anônimos (imutáveis).

## Arquitetura de Sistema (System Design)

### O Motor de Geração de IDs

Para garantir que o sistema não sofra indisponibilidade por lentidão e que atenda a picos sem latência na geração de IDs, os identificadores seguirão uma lógica de **Reserva em Lote**:

1.  A aplicação reservará um intervalo de números no coordenador (Redis-ID) (ex: de 1.000.000 a 1.001.000).
2.  Esses IDs serão consumidos localmente pela instância da API.
3.  Apenas quando o lote de 1.000 terminar, uma nova chamada de rede será feita ao coordenador.

### O Conceito de "Eras" e Ofuscação

Para evitar que URLs curtas pareçam previsíveis, o sistema utilizará:

  * **Base62 Customizada:** Os números serão convertidos para texto usando o alfabeto (0-9, A-Z, a-z). Este alfabeto será embaralhado via Seed, injetada através de variáveis de ambiente, garantindo que IDs sequenciais não gerem resultados visualmente lineares.

  * **Eras de Escalonamento:** O sistema começará com IDs que resultarão em strings de 4 caracteres. Ao atingir um limite pré-definido, migrará para a próxima "Era", aumentando o tamanho da string de forma controlada.

  * **Multiplicador Primo e Permutação Pseudo-Aleatória:** O ID sequencial do lote será multiplicado por um número primo específico daquela Era. Para garantir que o valor final gerado esteja estritamente dentro dos limites da Era, será utilizada a fórmula abaixo, onde o ID inicial é múltiplicado por um número primo, e então o resultado do módulo deste valor pelo tamanho do intervalo é somado ao ID de início daquela Era:

    $$\text{ID}_{\text{final}} = ((\text{ID}_{\text{sequencial}} \times \text{Primo}_{\text{da Era}}) \bmod \text{Tamanho}_{\text{da Era}}) + \text{Início}_{\text{da Era}}$$

    Isso garantirá que IDs vizinhos (ex: 1 e 2) resultem em valores finais muito distantes um do outro, parecendo aleatórios para potenciais atacantes.

## Estratégia de Dados e Armazenamento

O sistema utilizará três tecnologias de armazenamento de forma especializada:

| Tecnologia           | Papel Principal       | Justificativa                                                                                                                                                                                                                                           |
| :------------------- | :-------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **PostgreSQL**       | Backoffice e Usuários | Garantirá a integridade referencial para contas e pagamentos.                                                                                                                                                                                           |
| **Apache Cassandra** | Core de Links e Logs  | Suportará milhares de registros de cliques com escrita ultrarrápida e sem gargalos de travamento (locks).                                                                                                                                               |
| **Redis (Duplo)**    | Cache e Coordenação   | **Instância A (Cache):** Garantirá rapidez no redirecionamento, utilizando política de evicção LRU (Least Recently Used) e TTL estendido (ex: 7 dias) para otimização da memória.<br>**Instância B (ID):** Atuará como contador atômico para unicidade. |

## Processamento Assíncrono (Event-Driven)

O redirecionamento de um link não poderá esperar o banco de dados registrar quem clicou. Por isso, será utilizado o **RabbitMQ**:

1.  O usuário clicará no link.
2.  A API enviará o redirecionamento (302) imediatamente.
3.  Um "evento de clique" será publicado em uma fila.
4.  **Workers** em segundo plano lerão essa fila e atualizarão o Cassandra e o Postgres de forma assíncrona, sem travar a experiência do usuário.

## Metas de Performance (SLO)

  * **Latência de Redirecionamento:** 99% das requisições deverão ser resolvidas em menos de **15ms**.
  * **Perda de Dados:** A tolerância de erro no Analytics deverá ser de menos de **0,1%**, garantida pelo sistema de confirmação (ACK) da mensageria.

## Segurança e Prevenção de Abuso

Como encurtadores de URL são alvos frequentes, o sistema implementará camadas de proteção nativas:

  * **Rate Limiting:** Utilizará algoritmos (*Token Bucket* ou *Sliding Window*) via Redis para limitar a criação de links por IP (ex: 10 links/minuto para usuários anônimos), mitigando abusos e spam.
  * **Validação de Destino:** Será integrado com APIs de navegação segura (como Google Safe Browsing) para verificar a URL final antes de permitir a criação do link, bloqueando a disseminação de *phishing* ou *malware*.

## Backend API (Go)

O motor principal do sistema, responsável pelo roteamento, geração de IDs e regras de negócio, será desenvolvido em **Golang**. A escolha da linguagem alinha-se diretamente com as metas rigorosas de performance, escalabilidade e resiliência do projeto.

### Arquitetura e Decisões Técnicas
* **Alta Concorrência:** Utilização nativa de *Goroutines* e *Channels* para processar milhares de requisições de redirecionamento simultâneas com uso mínimo de memória (overhead).
* **Eficiência de Rede:** Aproveitamento do ecossistema Go para conexões ultrarrápidas e persistentes com a infraestrutura core (Redis, RabbitMQ, Cassandra e PostgreSQL).
* **Deploy Otimizado:** Compilação da aplicação em um binário estático único, gerando contêineres Docker extremamente leves. Isso garantirá um tempo de inicialização quase instantâneo (*fast cold start*), ideal para regras de *auto-scaling* agressivas em picos de tráfego.

## App Mobile (Flutter)

Será implementado um aplicativo para Android, este será desenhado para ser uma ferramenta de produtividade:

  * **Sincronização Unidirecional:** O app baixará o histórico do servidor para o banco local (SQLite).
  * **Funcionalidades Offline:** Permitirá que o usuário consulte seus links, gere QR Codes e veja métricas básicas sem internet.
  * **Criação Restrita:** Novos links só poderão ser criados online para garantir a reserva global de IDs.

## Frontend Web (JavaScript)

A interface web atuará como o principal portal de conversão para usuários premium e oferecerá a experiência de criação rápida para usuários anônimos. A ser desenvolvida em JavaScript/TypeScript, a aplicação será totalmente desacoplada, consumindo a mesma API RESTful utilizada pelo aplicativo mobile.

### Arquitetura e Decisões Técnicas

  * **Abordagem Híbrida (SSR / CSR):**
      * **Páginas Institucionais e SEO:** A página inicial e as rotas de marketing serão renderizadas no servidor (SSR) para garantir indexação otimizada por motores de busca e carregamento inicial quase instantâneo.
      * **Dashboard Premium (SPA):** A área logada operará como uma *Single Page Application* (CSR), focando em interatividade rica e transições fluidas para a visualização de gráficos e métricas (Analytics).
  * **Gerenciamento de Estado e Cache de UI:** Utilizará bibliotecas de *data-fetching* (como React Query ou SWR) para fazer cache de requisições, evitando chamadas redundantes à API ao navegar entre as métricas de diferentes links.
  * **Segurança no Client-Side:**
      * Os tokens JWT serão armazenados em cookies `HttpOnly` e `Secure` para mitigar ataques XSS.
      * Haverá validação rigorosa de formulários e *debounce* na criação de URLs para interagir com o *Rate Limiting* da API de forma amigável ao usuário.

## Roadmap de Implementação

| Status | Fase / Módulo                | Entregável Principal       | Descrição                                                                                          |
| :----: | :--------------------------- | :------------------------- | :------------------------------------------------------------------------------------------------- |
| `[X]`  | **Fase 0: Requisitos**       | Documentação dos Objetivos | Desenvolver e criar a documentação a ser seguida durante a implementação desse projeto.            |
| `[ ]`  | **Fase 1: Armazenamento**    | Modelagem de dados Inicial | Definir a estrutura e esquemas iniciais no PostgreSQL e Cassandra para preparar a base do sistema. |
| `[ ]`  | **Fase 2: Core**             | Motor de Geração de IDs    | Implementar reserva em lote no Redis e a ofuscação com Base62 e multiplicador primo.               |
| `[ ]`  | **Fase 3: Infra**            | Redirecionamento e Filas   | Criar o endpoint de redirecionamento (SLO < 15ms) e a publicação de eventos no RabbitMQ.           |
| `[ ]`  | **Fase 4: Armazenamento**    | Modelagem de Dados         | Configurar o cluster Apache Cassandra para logs de cliques e PostgreSQL para usuários.             |
| `[ ]`  | **Fase 5: Armazenamento**    | Workers Assíncronos        | Desenvolver os consumidores da fila para consolidar os dados analíticos de forma assíncrona.       |
| `[ ]`  | **Fase 6: Frontend Web**     | Landing Page SSR           | Desenvolver as páginas institucionais híbridas com foco em SEO e carregamento rápido.              |
| `[ ]`  | **Fase 7: Frontend Web**     | Dashboard Premium (SPA)    | Criar a interface logada, exibição de gráficos de Analytics e gerenciamento de JWT.                |
| `[ ]`  | **Fase 8: Mobile & Offline** | App Flutter Base           | Implementar a interface principal do aplicativo e a comunicação restrita de criação via API.       |
| `[ ]`  | **Fase 9: Mobile & Offline** | Sincronização Local        | Desenvolver a lógica offline-first no SQLite para acesso ao histórico e QR Codes sem internet.     |
| `[ ]`  | **Fase 10: Segurança**       | Proteção de Borda          | Integrar algoritmos de Rate Limiting e a validação via Google Safe Browsing.                       |
