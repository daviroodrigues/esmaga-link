package email

import (
	"context"
	"strings"
	"testing"

	"esmaga-link/internal/domain"
)

type stubStore struct {
	templates map[string]domain.EmailTemplate
}

func (s *stubStore) GetBySlug(_ context.Context, slug string) (*domain.EmailTemplate, error) {
	if t, ok := s.templates[slug]; ok {
		return &t, nil
	}
	return nil, nil
}

func TestRenderer_AllTemplates(t *testing.T) {
	r := NewRenderer(nil)

	cases := []struct {
		name     string
		vars     map[string]string
		wantSubj string
		wantText string
	}{
		{
			name:     domain.TemplateEmailVerification,
			vars:     map[string]string{"name": "Davi", "action_url": "https://esmaga.link/verify?token=abc"},
			wantSubj: "Confirme seu e-mail",
			wantText: "https://esmaga.link/verify?token=abc",
		},
		{
			name:     domain.TemplatePasswordReset,
			vars:     map[string]string{"action_url": "https://esmaga.link/reset?token=xyz"},
			wantSubj: "Redefinição de senha",
			wantText: "https://esmaga.link/reset?token=xyz",
		},
		{
			name:     domain.TemplateAccountDeleted,
			vars:     map[string]string{},
			wantSubj: "encerrada",
			wantText: "encerrada",
		},
	}

	for _, tc := range cases {
		subj, htmlBody, textBody, err := r.Render(context.Background(), tc.name, tc.vars)
		if err != nil {
			t.Fatalf("template %s falhou: %v", tc.name, err)
		}
		if !strings.Contains(subj, tc.wantSubj) {
			t.Errorf("template %s: assunto %q não contém %q", tc.name, subj, tc.wantSubj)
		}
		if !strings.Contains(htmlBody, tc.wantText) {
			t.Errorf("template %s: HTML não contém %q", tc.name, tc.wantText)
		}
		if !strings.Contains(textBody, tc.wantText) {
			t.Errorf("template %s: texto não contém %q", tc.name, tc.wantText)
		}
		for _, token := range []string{"#faf9f6", "#0f5c47", "#181715", "#e2dfd7", "prefers-color-scheme: dark", "Esmaga Link"} {
			if !strings.Contains(htmlBody, token) {
				t.Errorf("template %s: HTML não contém token de identidade visual %q", tc.name, token)
			}
		}
		if strings.Contains(htmlBody, "#4f46e5") {
			t.Errorf("template %s: HTML ainda contém cor indigo antiga #4f46e5", tc.name)
		}
	}
}

func TestRenderer_UsesDatabaseStoreFirst(t *testing.T) {
	store := &stubStore{
		templates: map[string]domain.EmailTemplate{
			domain.TemplateEmailVerification: {
				Slug:     domain.TemplateEmailVerification,
				Subject:  "Assunto Customizado do Banco",
				HTMLBody: "<p>Custom {{.ActionURL}}</p>",
				TextBody: "Custom {{.ActionURL}}",
			},
		},
	}
	r := NewRenderer(store)

	subj, htmlBody, textBody, err := r.Render(context.Background(), domain.TemplateEmailVerification, map[string]string{
		"action_url": "https://esmaga.link/v/123",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if subj != "Assunto Customizado do Banco" {
		t.Errorf("esperava assunto do banco, obteve %q", subj)
	}
	if htmlBody != "<p>Custom https://esmaga.link/v/123</p>" || textBody != "Custom https://esmaga.link/v/123" {
		t.Errorf("corpo renderizado inesperado: html=%q text=%q", htmlBody, textBody)
	}
}

func TestBuildMIMEMessage(t *testing.T) {
	raw := string(buildMIMEMessage("from@example.com", "to@example.com", "Teste", "ola texto", "<p>ola html</p>"))
	for _, part := range []string{"From: from@example.com", "To: to@example.com", "multipart/alternative", "ola texto", "<p>ola html</p>"} {
		if !strings.Contains(raw, part) {
			t.Errorf("MIME não contém %q", part)
		}
	}
}
