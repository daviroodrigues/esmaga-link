package cassandra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

var (
	ErrCodeCollision = errors.New("colisão detectada: código de link já existente no cassandra")
)

type LinkRepository struct {
	session *gocql.Session
}

func NewLinkRepository(session *gocql.Session) *LinkRepository {
	return &LinkRepository{session: session}
}

func (r *LinkRepository) Create(ctx context.Context, l *domain.Link) error {
	var userUUID *gocql.UUID
	if l.UserID != nil {
		u, err := gocql.ParseUUID(l.UserID.String())
		if err == nil {
			userUUID = &u
		}
	}

	queryCode := `
		INSERT INTO links_by_code (short_code, original_url, user_id, is_active, created_at)
		VALUES (?, ?, ?, ?, ?)
		IF NOT EXISTS
	`
	applied, err := r.session.Query(queryCode, l.ShortCode, l.OriginalURL, userUUID, l.IsActive, l.CreatedAt).
		WithContext(ctx).
		MapScanCAS(make(map[string]interface{}))

	if err != nil {
		return fmt.Errorf("erro ao inserir links_by_code no cassandra: %w", err)
	}
	if !applied {
		return ErrCodeCollision
	}

	if userUUID != nil {
		queryUser := `
			INSERT INTO links_by_user (user_id, created_at, short_code, original_url, is_active)
			VALUES (?, ?, ?, ?, ?)
		`
		if err := r.session.Query(queryUser, *userUUID, l.CreatedAt, l.ShortCode, l.OriginalURL, l.IsActive).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("erro ao inserir links_by_user no cassandra: %w", err)
		}
	}

	return nil
}

func (r *LinkRepository) GetCheckpoint(ctx context.Context, sequenceName string) (int64, error) {
	query := `SELECT last_allocated_id FROM id_checkpoints WHERE sequence_name = ?`
	var lastID int64
	if err := r.session.Query(query, sequenceName).WithContext(ctx).Scan(&lastID); err != nil {
		if err == gocql.ErrNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("erro ao ler checkpoint no cassandra: %w", err)
	}
	return lastID, nil
}

func (r *LinkRepository) SaveCheckpoint(ctx context.Context, sequenceName string, allocatedID int64) error {
	query := `
		INSERT INTO id_checkpoints (sequence_name, last_allocated_id, updated_at)
		VALUES (?, ?, ?)
	`
	if err := r.session.Query(query, sequenceName, allocatedID, time.Now().UTC()).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("erro ao salvar checkpoint no cassandra: %w", err)
	}
	return nil
}

func (r *LinkRepository) GetByCode(ctx context.Context, shortCode string) (*domain.Link, error) {
	query := `
		SELECT short_code, original_url, user_id, is_active, created_at
		FROM links_by_code
		WHERE short_code = ?
	`
	var (
		code      string
		url       string
		userUUID  *gocql.UUID
		isActive  bool
		createdAt time.Time
	)

	err := r.session.Query(query, shortCode).Consistency(gocql.LocalOne).WithContext(ctx).
		Scan(&code, &url, &userUUID, &isActive, &createdAt)
	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar link por code: %w", err)
	}

	var parsedUserID *uuid.UUID
	if userUUID != nil {
		u, parseErr := uuid.Parse(userUUID.String())
		if parseErr == nil {
			parsedUserID = &u
		}
	}

	return &domain.Link{
		ShortCode:   code,
		OriginalURL: url,
		UserID:      parsedUserID,
		IsActive:    isActive,
		CreatedAt:   createdAt,
	}, nil
}

func (r *LinkRepository) GetByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Link, error) {
	u, err := gocql.ParseUUID(userID.String())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT short_code, original_url, is_active, created_at
		FROM links_by_user
		WHERE user_id = ?
	`
	iter := r.session.Query(query, u).WithContext(ctx).Iter()

	var links []*domain.Link
	var (
		code      string
		url       string
		isActive  bool
		createdAt time.Time
	)

	for iter.Scan(&code, &url, &isActive, &createdAt) {
		uid := userID
		links = append(links, &domain.Link{
			ShortCode:   code,
			OriginalURL: url,
			UserID:      &uid,
			IsActive:    isActive,
			CreatedAt:   createdAt,
		})
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("erro ao iterar links do usuário: %w", err)
	}

	return links, nil
}

func (r *LinkRepository) UpdateURL(ctx context.Context, shortCode string, userID uuid.UUID, newURL string) error {
	u, err := gocql.ParseUUID(userID.String())
	if err != nil {
		return err
	}

	queryCode := `UPDATE links_by_code SET original_url = ? WHERE short_code = ?`
	if err := r.session.Query(queryCode, newURL, shortCode).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("erro ao atualizar links_by_code: %w", err)
	}

	var createdAt time.Time
	err = r.session.Query(`SELECT created_at FROM links_by_code WHERE short_code = ?`, shortCode).WithContext(ctx).Scan(&createdAt)
	if err == nil {
		queryUser := `UPDATE links_by_user SET original_url = ? WHERE user_id = ? AND created_at = ? AND short_code = ?`
		_ = r.session.Query(queryUser, newURL, u, createdAt, shortCode).WithContext(ctx).Exec()
	}

	return nil
}

func (r *LinkRepository) DeactivateLink(ctx context.Context, shortCode string, userID uuid.UUID, createdAt time.Time) error {
	u, err := gocql.ParseUUID(userID.String())
	if err != nil {
		return err
	}

	queryCode := `UPDATE links_by_code SET is_active = false WHERE short_code = ?`
	if err := r.session.Query(queryCode, shortCode).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("erro ao desativar link em links_by_code: %w", err)
	}

	queryUser := `UPDATE links_by_user SET is_active = false WHERE user_id = ? AND created_at = ? AND short_code = ?`
	if err := r.session.Query(queryUser, u, createdAt, shortCode).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("erro ao desativar link em links_by_user: %w", err)
	}

	return nil
}

func (r *LinkRepository) AnonymizeUserLinks(ctx context.Context, userID uuid.UUID) error {
	links, err := r.GetByUser(ctx, userID)
	if err != nil {
		return err
	}

	u, _ := gocql.ParseUUID(userID.String())

	for _, l := range links {

		queryCode := `UPDATE links_by_code SET user_id = null WHERE short_code = ?`
		_ = r.session.Query(queryCode, l.ShortCode).WithContext(ctx).Exec()

		queryUser := `DELETE FROM links_by_user WHERE user_id = ? AND created_at = ? AND short_code = ?`
		_ = r.session.Query(queryUser, u, l.CreatedAt, l.ShortCode).WithContext(ctx).Exec()
	}

	return nil
}

func (r *LinkRepository) DeactivateUserLinks(ctx context.Context, userID uuid.UUID) ([]string, error) {
	links, err := r.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	u, _ := gocql.ParseUUID(userID.String())
	var codes []string

	for _, l := range links {
		codes = append(codes, l.ShortCode)

		queryCode := `UPDATE links_by_code SET is_active = false WHERE short_code = ?`
		_ = r.session.Query(queryCode, l.ShortCode).WithContext(ctx).Exec()

		queryUser := `DELETE FROM links_by_user WHERE user_id = ? AND created_at = ? AND short_code = ?`
		_ = r.session.Query(queryUser, u, l.CreatedAt, l.ShortCode).WithContext(ctx).Exec()
	}

	return codes, nil
}

func (r *LinkRepository) InsertClickEvent(ctx context.Context, c *domain.ClickEvent) error {
	bucketDate := c.ClickedAt.UTC().Format("2006-01-02")

	timeUUID := gocql.TimeUUID()

	query := `
		INSERT INTO click_events (
			short_code, bucket_date, clicked_at, event_id, ip_address, user_agent, referrer, country, device_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	err := r.session.Query(query,
		c.ShortCode,
		bucketDate,
		c.ClickedAt,
		timeUUID,
		c.IPAddress,
		c.UserAgent,
		c.Referrer,
		c.Country,
		c.DeviceType,
	).WithContext(ctx).Exec()

	if err != nil {
		return fmt.Errorf("erro ao inserir evento de clique no cassandra: %w", err)
	}

	return nil
}

func (r *LinkRepository) GetAnalyticsByCode(ctx context.Context, shortCode string, days int) (*domain.LinkAnalytics, error) {
	if days <= 0 {
		days = 7
	}

	analytics := &domain.LinkAnalytics{
		ShortCode:       shortCode,
		ClicksByDate:    make(map[string]int64),
		ClicksByDevice:  make(map[string]int64),
		ClicksByCountry: make(map[string]int64),
		TopReferrers:    make(map[string]int64),
	}

	query := `
		SELECT ip_address, user_agent, referrer, country, device_type
		FROM click_events
		WHERE short_code = ? AND bucket_date = ?
	`

	now := time.Now().UTC()
	for i := 0; i < days; i++ {
		dateStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		iter := r.session.Query(query, shortCode, dateStr).WithContext(ctx).Iter()

		var (
			ip         string
			ua         string
			ref        string
			country    string
			deviceType string
		)

		for iter.Scan(&ip, &ua, &ref, &country, &deviceType) {
			analytics.TotalClicks++
			analytics.ClicksByDate[dateStr]++

			if deviceType == "" {
				deviceType = "unknown"
			}
			analytics.ClicksByDevice[deviceType]++

			if country != "" {
				analytics.ClicksByCountry[country]++
			}

			if ref != "" {
				analytics.TopReferrers[ref]++
			}
		}

		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("erro ao ler analytics do cassandra: %w", err)
		}
	}

	return analytics, nil
}
