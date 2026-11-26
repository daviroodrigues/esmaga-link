package email

import (
	"bytes"
	"context"
	"fmt"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"

	"esmaga-link/internal/domain"
)

type TemplateStore interface {
	GetBySlug(ctx context.Context, slug string) (*domain.EmailTemplate, error)
}

type emailLayout struct {
	Title       string
	Section     string
	Preheader   string
	Heading     string
	BodyHTML    string
	ActionLabel string
	NoticeHTML  string
	FooterNote  string
}

func buildEmailHTML(l emailLayout) string {
	var actionBlock string
	if l.ActionLabel != "" {
		actionBlock = fmt.Sprintf(`
                      <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 22px 0;">
                        <tr>
                          <td class="btn-primary" align="center" bgcolor="#0f5c47" style="background-color:#0f5c47;border-radius:6px;">
                            <a href="{{.ActionURL}}" target="_blank" rel="noopener noreferrer" class="btn-primary-link" style="display:inline-block;padding:12px 22px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;font-size:14px;font-weight:600;line-height:1.2;color:#ffffff;text-decoration:none;border-radius:6px;">%s</a>
                          </td>
                        </tr>
                      </table>
                      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" class="surface-alt" style="background-color:#f3f1ec;border:1px solid #e2dfd7;border-radius:6px;margin:0 0 20px 0;">
                        <tr>
                          <td style="padding:12px 14px;">
                            <p class="text-muted" style="margin:0 0 6px 0;font-size:12px;line-height:1.45;color:#5c5852;">Se o botão não funcionar, copie e cole o endereço abaixo no navegador:</p>
                            <p style="margin:0;font-family:ui-monospace,SFMono-Regular,'SF Mono',Menlo,Monaco,Consolas,monospace;font-size:12px;line-height:1.5;word-break:break-all;">
                              <a href="{{.ActionURL}}" target="_blank" rel="noopener noreferrer" class="accent-link" style="color:#0c4f3d;text-decoration:underline;">{{.ActionURL}}</a>
                            </p>
                          </td>
                        </tr>
                      </table>`, l.ActionLabel)
	}

	var noticeBlock string
	if l.NoticeHTML != "" {
		noticeBlock = fmt.Sprintf(`
                      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" class="callout" style="background-color:#edf4f1;border:1px solid #e2dfd7;border-left:3px solid #0f5c47;border-radius:6px;margin:0 0 18px 0;">
                        <tr>
                          <td class="callout-text" style="padding:12px 14px;font-size:13px;line-height:1.5;color:#181715;">%s</td>
                        </tr>
                      </table>`, l.NoticeHTML)
	}

	var footerNoteBlock string
	if l.FooterNote != "" {
		footerNoteBlock = fmt.Sprintf(`
                      <p class="text-muted" style="margin:0;font-size:13px;line-height:1.5;color:#5c5852;">%s</p>`, l.FooterNote)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="color-scheme" content="light dark">
  <meta name="supported-color-schemes" content="light dark">
  <title>%s</title>
  <style>
    :root { color-scheme: light dark; supported-color-schemes: light dark; }
    @media (prefers-color-scheme: dark) {
      .email-bg { background-color: #111110 !important; }
      .email-card { background-color: #191917 !important; border-color: #2e2d29 !important; }
      .email-divider { border-color: #2e2d29 !important; }
      .text-primary { color: #f2f1ed !important; }
      .text-muted { color: #9e9b93 !important; }
      .brand-sep { color: #46443e !important; }
      .brand-badge { background-color: #2fbf8c !important; color: #062017 !important; }
      .btn-primary { background-color: #2fbf8c !important; }
      .btn-primary-link { color: #062017 !important; }
      .surface-alt { background-color: #22211f !important; border-color: #2e2d29 !important; }
      .accent-link { color: #4fd1a5 !important; }
      .callout { background-color: #16241e !important; border-color: #2e2d29 !important; border-left-color: #2fbf8c !important; }
      .callout-text { color: #f2f1ed !important; }
    }
  </style>
</head>
<body class="email-bg" style="margin:0;padding:0;background-color:#faf9f6;color:#181715;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;mso-hide:all;">%s</div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" class="email-bg" style="background-color:#faf9f6;">
    <tr>
      <td align="center" style="padding:32px 16px 40px 16px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:520px;margin:0 auto;">
          <tr>
            <td style="padding:0 0 18px 0;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td class="brand-badge" width="28" height="28" align="center" valign="middle" bgcolor="#0f5c47" style="width:28px;height:28px;border-radius:6px;background-color:#0f5c47;color:#ffffff;font-size:14px;font-weight:700;line-height:28px;letter-spacing:-0.02em;">E</td>
                  <td style="padding-left:10px;vertical-align:middle;">
                    <span class="text-primary" style="font-size:16px;font-weight:700;letter-spacing:-0.02em;color:#181715;">Esmaga Link</span>
                    <span class="brand-sep" style="padding:0 6px;font-size:14px;color:#c7c2b8;">/</span>
                    <span class="text-muted" style="font-size:14px;font-weight:500;color:#5c5852;">%s</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td class="email-card" bgcolor="#ffffff" style="background-color:#ffffff;border:1px solid #e2dfd7;border-radius:8px;padding:28px 28px 26px 28px;">
              <h1 class="text-primary" style="margin:0 0 12px 0;font-size:20px;font-weight:700;letter-spacing:-0.02em;line-height:1.3;color:#181715;">%s</h1>
%s%s%s%s
            </td>
          </tr>
          <tr>
            <td class="email-divider" style="padding:20px 4px 0 4px;">
              <p class="text-muted" style="margin:0;font-size:12px;line-height:1.5;color:#5c5852;">Esmaga Link — Encurtador de URLs.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, l.Title, l.Preheader, l.Section, l.Heading, l.BodyHTML, actionBlock, noticeBlock, footerNoteBlock)

	return strings.TrimSpace(html)
}

var defaultTemplatesList = []domain.EmailTemplate{
	{
		Slug:    domain.TemplateEmailVerification,
		Subject: "Confirme seu e-mail — Esmaga Link",
		HTMLBody: buildEmailHTML(emailLayout{
			Title:     "Confirme seu e-mail — Esmaga Link",
			Section:   "Verificação",
			Preheader: "Confirme seu endereço de e-mail para ativar todos os recursos da sua conta no Esmaga Link.",
			Heading:   "Confirme seu e-mail",
			BodyHTML: `              <p class="text-primary" style="margin:0 0 12px 0;font-size:15px;line-height:1.6;color:#181715;">Olá{{if .Name}}, {{.Name}}{{end}}!</p>
              <p class="text-muted" style="margin:0;font-size:15px;line-height:1.6;color:#5c5852;">Confirme seu endereço de e-mail para ativar todos os recursos da sua conta no Esmaga Link, como editar destinos e acompanhar cliques no painel.</p>`,
			ActionLabel: "Confirmar e-mail",
			FooterNote:  "Este link expira em 24 horas. Se você não criou esta conta, pode ignorar este e-mail.",
		}),
		TextBody: `Esmaga Link — Confirme seu e-mail

Olá{{if .Name}}, {{.Name}}{{end}}!

Confirme seu endereço de e-mail para ativar todos os recursos da sua conta acessando o link abaixo:
{{.ActionURL}}

Este link expira em 24 horas. Se você não criou esta conta, pode ignorar este e-mail.

--
Esmaga Link — Encurtador de URLs.`,
	},
	{
		Slug:    domain.TemplatePasswordReset,
		Subject: "Redefinição de senha — Esmaga Link",
		HTMLBody: buildEmailHTML(emailLayout{
			Title:     "Redefinição de senha — Esmaga Link",
			Section:   "Segurança",
			Preheader: "Recebemos uma solicitação para redefinir a senha da sua conta no Esmaga Link.",
			Heading:   "Redefinir senha",
			BodyHTML: `              <p class="text-muted" style="margin:0;font-size:15px;line-height:1.6;color:#5c5852;">Recebemos uma solicitação para redefinir a senha da sua conta no Esmaga Link. Use o botão abaixo para escolher uma nova senha.</p>`,
			ActionLabel: "Redefinir senha",
			NoticeHTML:  "Este link expira em <strong>30 minutos</strong>. Se você não solicitou a redefinição, pode ignorar este e-mail com segurança — sua senha atual permanece inalterada.",
		}),
		TextBody: `Esmaga Link — Redefinição de senha

Recebemos uma solicitação para redefinir a senha da sua conta no Esmaga Link.

Acesse o link abaixo para criar uma nova senha:
{{.ActionURL}}

Este link expira em 30 minutos. Se você não solicitou a redefinição, ignore este e-mail.

--
Esmaga Link — Encurtador de URLs.`,
	},
	{
		Slug:    domain.TemplateAccountDeleted,
		Subject: "Sua conta foi encerrada — Esmaga Link",
		HTMLBody: buildEmailHTML(emailLayout{
			Title:     "Sua conta foi encerrada — Esmaga Link",
			Section:   "Conta",
			Preheader: "Confirmamos que sua conta no Esmaga Link foi encerrada conforme solicitado.",
			Heading:   "Sua conta foi encerrada",
			BodyHTML: `              <p class="text-muted" style="margin:0 0 20px 0;font-size:15px;line-height:1.6;color:#5c5852;">Confirmamos que sua conta no Esmaga Link foi encerrada conforme solicitado e seus dados de acesso foram removidos.</p>`,
			NoticeHTML: "Se você optou por manter seus links ativos, eles continuam funcionando normalmente de forma anônima e imutável.",
			FooterNote: "Obrigado por ter usado o Esmaga Link.",
		}),
		TextBody: `Esmaga Link — Conta encerrada

Confirmamos que sua conta no Esmaga Link foi encerrada conforme solicitado e seus dados de acesso foram removidos.

Se você optou por manter seus links ativos, eles continuam funcionando normalmente de forma anônima e imutável.

Obrigado por ter usado o Esmaga Link.

--
Esmaga Link — Encurtador de URLs.`,
	},
}

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
