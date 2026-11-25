package email

import (
	"bytes"
	"context"
	"fmt"
	htmltmpl "html/template"
	texttmpl "text/template"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
)

// TemplateStore abstrai a leitura dos templates de e-mail persistidos no banco.
type TemplateStore interface {
	GetBySlug(ctx context.Context, slug string) (*domain.EmailTemplate, error)
}

var defaultTemplatesList = []domain.EmailTemplate{
	{
		Slug:    domain.TemplateEmailVerification,
		Subject: "Confirme seu e-mail - Esmaga Link",
		HTMLBody: `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="UTF-8"><title>Confirme seu e-mail</title></head>
<body style="font-family:sans-serif;background:#f8fafc;color:#0f172a;padding:24px;">
  <div style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;border:1px solid #e2e8f0;">
    <h2 style="margin-top:0;color:#4f46e5;">Esmaga Link</h2>
    <p>Olá{{if .Name}}, {{.Name}}{{end}}!</p>
    <p>Confirme seu endereço de e-mail para ativar todos os recursos da sua conta:</p>
    <p style="margin:24px 0;">
      <a href="{{.ActionURL}}" style="background:#4f46e5;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:6px;display:inline-block;font-weight:600;">Confirmar E-mail</a>
    </p>
    <p style="font-size:13px;color:#64748b;">Se o botão não funcionar, copie e cole o link abaixo no navegador:<br>{{.ActionURL}}</p>
    <p style="font-size:12px;color:#94a3b8;">Este link expira em 24 horas.</p>
  </div>
</body>
</html>`,
		TextBody: `Esmaga Link - Confirme seu e-mail

Olá{{if .Name}}, {{.Name}}{{end}}!

Confirme seu endereço de e-mail acessando o link abaixo:
{{.ActionURL}}

Este link expira em 24 horas.`,
	},
	{
		Slug:    domain.TemplatePasswordReset,
		Subject: "Redefinição de senha - Esmaga Link",
		HTMLBody: `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="UTF-8"><title>Redefinição de senha</title></head>
<body style="font-family:sans-serif;background:#f8fafc;color:#0f172a;padding:24px;">
  <div style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;border:1px solid #e2e8f0;">
    <h2 style="margin-top:0;color:#4f46e5;">Esmaga Link</h2>
    <p>Recebemos uma solicitação para redefinir a senha da sua conta.</p>
    <p style="margin:24px 0;">
      <a href="{{.ActionURL}}" style="background:#4f46e5;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:6px;display:inline-block;font-weight:600;">Redefinir Senha</a>
    </p>
    <p style="font-size:13px;color:#64748b;">Se o botão não funcionar, copie e cole o link abaixo no navegador:<br>{{.ActionURL}}</p>
    <p style="font-size:12px;color:#94a3b8;">Este link expira em 30 minutos. Se você não solicitou, ignore este e-mail.</p>
  </div>
</body>
</html>`,
		TextBody: `Esmaga Link - Redefinição de senha

Recebemos uma solicitação para redefinir a senha da sua conta.

Acesse o link abaixo para criar uma nova senha:
{{.ActionURL}}

Este link expira em 30 minutos. Se você não solicitou, ignore este e-mail.`,
	},
	{
		Slug:    domain.TemplateAccountDeleted,
		Subject: "Sua conta foi encerrada - Esmaga Link",
		HTMLBody: `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="UTF-8"><title>Conta encerrada</title></head>
<body style="font-family:sans-serif;background:#f8fafc;color:#0f172a;padding:24px;">
  <div style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;border:1px solid #e2e8f0;">
    <h2 style="margin-top:0;color:#4f46e5;">Esmaga Link</h2>
    <p>Sua conta foi encerrada conforme solicitado.</p>
    <p style="font-size:13px;color:#64748b;">Se você optou por manter seus links ativos, eles continuam funcionando de forma anônima e imutável.</p>
  </div>
</body>
</html>`,
		TextBody: `Esmaga Link - Conta encerrada

Sua conta foi encerrada conforme solicitado.
Se você optou por manter seus links ativos, eles continuam funcionando de forma anônima e imutável.`,
	},
}

// DefaultTemplates retorna os templates padrão para seed inicial no PostgreSQL.
func DefaultTemplates() ([]domain.EmailTemplate, error) {
	out := make([]domain.EmailTemplate, len(defaultTemplatesList))
	copy(out, defaultTemplatesList)
	return out, nil
}

type templateData struct {
	Name      string
	ActionURL string
}

type Renderer struct {
	store    TemplateStore
	defaults map[string]domain.EmailTemplate
}

func NewRenderer(store TemplateStore) *Renderer {
	defaults := make(map[string]domain.EmailTemplate, len(defaultTemplatesList))
	for _, t := range defaultTemplatesList {
		defaults[t.Slug] = t
	}
	return &Renderer{
		store:    store,
		defaults: defaults,
	}
}

func (r *Renderer) Render(ctx context.Context, slug string, vars map[string]string) (subject, htmlBody, textBody string, err error) {
	var tmpl *domain.EmailTemplate
	if r.store != nil {
		tmpl, err = r.store.GetBySlug(ctx, slug)
		if err != nil {
			return "", "", "", err
		}
	}
	if tmpl == nil {
		def, ok := r.defaults[slug]
		if !ok {
			return "", "", "", fmt.Errorf("template de e-mail desconhecido: %s", slug)
		}
		tmpl = &def
	}

	data := templateData{
		Name:      vars["name"],
		ActionURL: vars["action_url"],
	}

	ht, err := htmltmpl.New(slug + ".html").Parse(tmpl.HTMLBody)
	if err != nil {
		return "", "", "", fmt.Errorf("erro ao compilar template HTML %s: %w", slug, err)
	}
	var htmlBuf bytes.Buffer
	if err := ht.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("erro ao renderizar template HTML %s: %w", slug, err)
	}

	tt, err := texttmpl.New(slug + ".txt").Parse(tmpl.TextBody)
	if err != nil {
		return "", "", "", fmt.Errorf("erro ao compilar template texto %s: %w", slug, err)
	}
	var textBuf bytes.Buffer
	if err := tt.Execute(&textBuf, data); err != nil {
		return "", "", "", fmt.Errorf("erro ao renderizar template texto %s: %w", slug, err)
	}

	return tmpl.Subject, htmlBuf.String(), textBuf.String(), nil
}
