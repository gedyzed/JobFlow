package emailsender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"github.com/gedyzed/JobFlow/JobService/services/worker"
	"github.com/resend/resend-go/v3"
)

const (
	// MaxResendBatchSize is the maximum number of emails allowed in a single Resend batch call.
	MaxResendBatchSize = 100

	defaultHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f4f4f7; color: #333333; margin: 0; padding: 20px; }
    .container { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05); }
    .header { background: #111827; padding: 24px; text-align: center; color: #ffffff; }
    .header h1 { margin: 0; font-size: 20px; font-weight: 600; letter-spacing: -0.025em; }
    .content { padding: 32px 24px; line-height: 1.6; font-size: 15px; color: #374151; }
    .footer { padding: 16px 24px; background-color: #f9fafb; font-size: 12px; color: #9ca3af; text-align: center; border-top: 1px solid #e5e7eb; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.Subject}}</h1>
    </div>
    <div class="content">
      <p>{{.Body}}</p>
    </div>
    <div class="footer">
      <p>Sent securely via {{.AppName}}</p>
    </div>
  </div>
</body>
</html>`
)

var defaultTmpl = template.Must(template.New("defaultEmail").Parse(defaultHTMLTemplate))

// ResendClient abstracts Resend API operations to allow unit testing and mocking.
type ResendClient interface {
	SendEmail(ctx context.Context, req *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
	SendBatch(ctx context.Context, batch []*resend.SendEmailRequest) (*resend.BatchEmailResponse, error)
}

type defaultResendClient struct {
	client *resend.Client
}

func (c *defaultResendClient) SendEmail(ctx context.Context, req *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	return c.client.Emails.SendWithContext(ctx, req)
}

func (c *defaultResendClient) SendBatch(ctx context.Context, batch []*resend.SendEmailRequest) (*resend.BatchEmailResponse, error) {
	return c.client.Batch.SendWithContext(ctx, batch)
}

type EmailSender struct {
	Config *configs.Config
	client ResendClient
}

func NewEmailSender(cfg *configs.Config) worker.ISendEmail {
	client := resend.NewClient(cfg.Email.APIKey)
	return &EmailSender{
		Config: cfg,
		client: &defaultResendClient{client: client},
	}
}

// NewEmailSenderWithClient allows injecting a custom or mock ResendClient.
func NewEmailSenderWithClient(cfg *configs.Config, client ResendClient) worker.ISendEmail {
	return &EmailSender{
		Config: cfg,
		client: client,
	}
}

func (e *EmailSender) SendEmail(ctx context.Context, payload interface{}) error {
	if payload == nil {
		return errors.New("email payload cannot be nil")
	}

	// 1. Check if payload is a batch of emails
	if batch, ok := toEmailBatch(payload); ok {
		return e.sendBatch(ctx, batch)
	}

	// 2. Process single email
	single, err := toSingleEmail(payload)
	if err != nil {
		return err
	}

	req, err := e.buildSendEmailRequest(single)
	if err != nil {
		return err
	}

	_, err = e.client.SendEmail(ctx, req)
	if err != nil {
		return errors.Wrap(err, "send email via resend")
	}

	return nil
}

func (e *EmailSender) sendBatch(ctx context.Context, batch []workerModels.SendEmailPayload) error {
	if len(batch) == 0 {
		return errors.New("email batch cannot be empty")
	}

	var requests []*resend.SendEmailRequest
	for i := range batch {
		req, err := e.buildSendEmailRequest(&batch[i])
		if err != nil {
			return errors.Wrapf(err, "invalid email at index %d in batch", i)
		}
		requests = append(requests, req)
	}

	// Chunk requests to satisfy Resend's batch limit (max 100 per request)
	for i := 0; i < len(requests); i += MaxResendBatchSize {
		end := i + MaxResendBatchSize
		if end > len(requests) {
			end = len(requests)
		}

		chunk := requests[i:end]
		_, err := e.client.SendBatch(ctx, chunk)
		if err != nil {
			return errors.Wrapf(err, "send email batch chunk [%d:%d] via resend", i, end)
		}
	}

	return nil
}

func (e *EmailSender) buildSendEmailRequest(p *workerModels.SendEmailPayload) (*resend.SendEmailRequest, error) {
	domain := "resend.dev"
	if e.Config != nil && strings.TrimSpace(e.Config.Email.Domain) != "" {
		domain = strings.TrimPrefix(strings.TrimSpace(e.Config.Email.Domain), "@")
	}

	if strings.TrimSpace(p.AppName) != "" {
		appName := strings.TrimSpace(p.AppName)
		if strings.Contains(appName, " ") {
			cleanName := strings.ReplaceAll(strings.ToLower(appName), " ", "")
			p.From = fmt.Sprintf("%s <%s@%s>", appName, cleanName, domain)
		} else {
			p.From = fmt.Sprintf("%s@%s", appName, domain)
		}
	} else if strings.TrimSpace(p.From) != "" {
		from := strings.TrimSpace(p.From)
		if !strings.Contains(from, "@") {
			p.From = fmt.Sprintf("%s@%s", from, domain)
		} else if strings.HasPrefix(from, "@") {
			p.From = fmt.Sprintf("jobflow%s", from)
		}
	}

	if strings.TrimSpace(p.From) == "" {
		return nil, errors.New("email 'from' address is required")
	}
	if len(p.To) == 0 {
		return nil, errors.New("email 'to' address is required")
	}
	if strings.TrimSpace(p.Subject) == "" {
		return nil, errors.New("email 'subject' is required")
	}

	htmlContent := p.Html
	if strings.TrimSpace(htmlContent) == "" {
		rendered, err := renderDefaultHTML(p.Subject, p.Body, p.Text, p.AppName)
		if err != nil {
			return nil, errors.Wrap(err, "render default email HTML template")
		}
		htmlContent = rendered
	}

	textContent := p.Text
	if textContent == "" {
		textContent = p.Body
	}

	var tags []resend.Tag
	for _, tag := range p.Tags {
		tags = append(tags, resend.Tag{
			Name:  tag.Name,
			Value: tag.Value,
		})
	}

	return &resend.SendEmailRequest{
		From:    p.From,
		To:      p.To,
		Subject: p.Subject,
		Html:    htmlContent,
		Text:    textContent,
		Cc:      p.Cc,
		Bcc:     p.Bcc,
		ReplyTo: p.ReplyTo,
		Tags:    tags,
	}, nil
}

func renderDefaultHTML(subject, body, text, appName string) (string, error) {
	content := body
	if content == "" {
		content = text
	}
	if content == "" {
		if appName != "" {
			content = fmt.Sprintf("Notification from %s", appName)
		} else {
			content = "Notification from JobFlow"
		}
	}

	footerName := appName
	if footerName == "" {
		footerName = "JobFlow"
	}

	data := struct {
		Subject string
		Body    string
		AppName string
	}{
		Subject: subject,
		Body:    content,
		AppName: footerName,
	}

	var buf bytes.Buffer
	if err := defaultTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func toEmailBatch(payload interface{}) ([]workerModels.SendEmailPayload, bool) {
	switch v := payload.(type) {
	case []workerModels.SendEmailPayload:
		return v, true
	case []*workerModels.SendEmailPayload:
		var batch []workerModels.SendEmailPayload
		for _, item := range v {
			if item != nil {
				batch = append(batch, *item)
			}
		}
		return batch, true
	case []byte:
		trimmed := strings.TrimSpace(string(v))
		if strings.HasPrefix(trimmed, "[") {
			var batch []workerModels.SendEmailPayload
			if err := json.Unmarshal(v, &batch); err == nil {
				return batch, true
			}
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "[") {
			var batch []workerModels.SendEmailPayload
			if err := json.Unmarshal([]byte(v), &batch); err == nil {
				return batch, true
			}
		}
	}
	return nil, false
}

func toSingleEmail(payload interface{}) (*workerModels.SendEmailPayload, error) {
	switch v := payload.(type) {
	case *workerModels.SendEmailPayload:
		return v, nil
	case workerModels.SendEmailPayload:
		return &v, nil
	case []byte:
		var p workerModels.SendEmailPayload
		if err := json.Unmarshal(v, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal JSON email payload")
		}
		return &p, nil
	case string:
		var p workerModels.SendEmailPayload
		if err := json.Unmarshal([]byte(v), &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal string JSON email payload")
		}
		return &p, nil
	default:
		// Map or arbitrary struct
		bytesData, err := json.Marshal(v)
		if err != nil {
			return nil, errors.Wrap(err, "marshal email payload")
		}
		var p workerModels.SendEmailPayload
		if err := json.Unmarshal(bytesData, &p); err != nil {
			return nil, errors.Wrap(err, "unmarshal email payload into SendEmailPayload")
		}
		return &p, nil
	}
}
