package emailsender

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"github.com/resend/resend-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockResendClient struct {
	sendEmailFunc func(ctx context.Context, req *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
	sendBatchFunc func(ctx context.Context, batch []*resend.SendEmailRequest) (*resend.BatchEmailResponse, error)

	sentRequests []*resend.SendEmailRequest
	sentBatches  [][]*resend.SendEmailRequest
}

func (m *mockResendClient) SendEmail(ctx context.Context, req *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	m.sentRequests = append(m.sentRequests, req)
	if m.sendEmailFunc != nil {
		return m.sendEmailFunc(ctx, req)
	}
	return &resend.SendEmailResponse{Id: "test-email-id"}, nil
}

func (m *mockResendClient) SendBatch(ctx context.Context, batch []*resend.SendEmailRequest) (*resend.BatchEmailResponse, error) {
	m.sentBatches = append(m.sentBatches, batch)
	if m.sendBatchFunc != nil {
		return m.sendBatchFunc(ctx, batch)
	}
	return &resend.BatchEmailResponse{
		Data: make([]resend.SendEmailResponse, len(batch)),
	}, nil
}

func TestSendEmail_SingleEmailWithDefaultTemplate(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	payload := workerModels.SendEmailPayload{
		From:    "Acme <onboarding@resend.dev>",
		To:      []string{"user@example.com"},
		Subject: "Welcome to JobFlow",
		Body:    "Your job processing is ready.",
		Tags: []workerModels.EmailTag{
			{Name: "category", Value: "welcome"},
		},
	}

	err := sender.SendEmail(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, mockClient.sentRequests, 1)

	req := mockClient.sentRequests[0]
	assert.Equal(t, "Acme <onboarding@resend.dev>", req.From)
	assert.Equal(t, []string{"user@example.com"}, req.To)
	assert.Equal(t, "Welcome to JobFlow", req.Subject)
	assert.Equal(t, "Your job processing is ready.", req.Text)
	// Default template check
	assert.Contains(t, req.Html, "Welcome to JobFlow")
	assert.Contains(t, req.Html, "Your job processing is ready.")
	assert.Contains(t, req.Html, "Sent securely via JobFlow")
	// Tags check
	require.Len(t, req.Tags, 1)
	assert.Equal(t, "category", req.Tags[0].Name)
	assert.Equal(t, "welcome", req.Tags[0].Value)
}

func TestSendEmail_SingleEmailWithCustomHTML(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	payload := &workerModels.SendEmailPayload{
		From:    "Acme <onboarding@resend.dev>",
		To:      []string{"user@example.com"},
		Subject: "Custom HTML",
		Html:    "<h1>Custom Header</h1><p>Custom Content</p>",
		Cc:      []string{"cc@example.com"},
		Bcc:     []string{"bcc@example.com"},
		ReplyTo: "reply@example.com",
	}

	err := sender.SendEmail(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, mockClient.sentRequests, 1)

	req := mockClient.sentRequests[0]
	assert.Equal(t, "<h1>Custom Header</h1><p>Custom Content</p>", req.Html)
	assert.Equal(t, []string{"cc@example.com"}, req.Cc)
	assert.Equal(t, []string{"bcc@example.com"}, req.Bcc)
	assert.Equal(t, "reply@example.com", req.ReplyTo)
}

func TestSendEmail_AppNameFormatting(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
			Domain: "resend.dev",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	// Test case 1: AppName provided without From -> appName@domain
	payload1 := workerModels.SendEmailPayload{
		AppName: "JobFlow",
		To:      []string{"user@example.com"},
		Subject: "Test AppName",
		Body:    "Body",
	}
	err := sender.SendEmail(context.Background(), payload1)
	require.NoError(t, err)
	assert.Equal(t, "JobFlow@resend.dev", mockClient.sentRequests[0].From)
	assert.Contains(t, mockClient.sentRequests[0].Html, "Sent securely via JobFlow")

	// Test case 2: AppName with space -> AppName <cleanname@domain>
	payload2 := workerModels.SendEmailPayload{
		AppName: "Job Flow",
		To:      []string{"user@example.com"},
		Subject: "Test AppName with space",
		Body:    "Body 2",
	}
	err = sender.SendEmail(context.Background(), payload2)
	require.NoError(t, err)
	assert.Equal(t, "Job Flow <jobflow@resend.dev>", mockClient.sentRequests[1].From)
	assert.Contains(t, mockClient.sentRequests[1].Html, "Sent securely via Job Flow")

	// Test case 3: Custom domain from config -> appName@customdomain
	customCfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
			Domain: "customdomain.com",
		},
	}
	customSender := NewEmailSenderWithClient(customCfg, mockClient)
	payload3 := workerModels.SendEmailPayload{
		AppName: "AcmeApp",
		To:      []string{"user@example.com"},
		Subject: "Test Custom Domain",
		Body:    "Body 3",
	}
	err = customSender.SendEmail(context.Background(), payload3)
	require.NoError(t, err)
	assert.Equal(t, "AcmeApp@customdomain.com", mockClient.sentRequests[2].From)
	assert.Contains(t, mockClient.sentRequests[2].Html, "Sent securely via AcmeApp")

	// Test case 4: Explicit From provided with empty AppName -> unchanged
	payload4 := workerModels.SendEmailPayload{
		From:    "alerts@example.com",
		To:      []string{"user@example.com"},
		Subject: "Test Explicit From",
		Body:    "Body 4",
	}
	err = sender.SendEmail(context.Background(), payload4)
	require.NoError(t, err)
	assert.Equal(t, "alerts@example.com", mockClient.sentRequests[3].From)
}

func TestSendEmail_BatchEmails_Chunking(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	// Create 150 emails to test chunking at 100 limit
	var batch []workerModels.SendEmailPayload
	for i := 1; i <= 150; i++ {
		batch = append(batch, workerModels.SendEmailPayload{
			From:    "sender@example.com",
			To:      []string{fmt.Sprintf("user%d@example.com", i)},
			Subject: fmt.Sprintf("Subject %d", i),
			Body:    fmt.Sprintf("Body %d", i),
		})
	}

	err := sender.SendEmail(context.Background(), batch)
	require.NoError(t, err)

	// Expected 2 batch calls: first with 100, second with 50
	require.Len(t, mockClient.sentBatches, 2)
	assert.Len(t, mockClient.sentBatches[0], 100)
	assert.Len(t, mockClient.sentBatches[1], 50)
	assert.Equal(t, "user1@example.com", mockClient.sentBatches[0][0].To[0])
	assert.Equal(t, "user101@example.com", mockClient.sentBatches[1][0].To[0])
}

func TestSendEmail_JSONPayloadParsing(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	// Test JSON bytes for single email
	jsonBytes := []byte(`{
		"from": "Acme <onboarding@resend.dev>",
		"to": ["json@example.com"],
		"subject": "JSON Test",
		"body": "Sent from raw json"
	}`)

	err := sender.SendEmail(context.Background(), jsonBytes)
	require.NoError(t, err)
	require.Len(t, mockClient.sentRequests, 1)
	assert.Equal(t, "json@example.com", mockClient.sentRequests[0].To[0])

	// Test JSON string for batch emails
	jsonBatchString := `[
		{
			"from": "Acme <onboarding@resend.dev>",
			"to": ["batch1@example.com"],
			"subject": "Batch 1",
			"body": "Batch body 1"
		},
		{
			"from": "Acme <onboarding@resend.dev>",
			"to": ["batch2@example.com"],
			"subject": "Batch 2",
			"body": "Batch body 2"
		}
	]`

	err = sender.SendEmail(context.Background(), jsonBatchString)
	require.NoError(t, err)
	require.Len(t, mockClient.sentBatches, 1)
	assert.Len(t, mockClient.sentBatches[0], 2)
}

func TestSendEmail_ValidationErrors(t *testing.T) {
	mockClient := &mockResendClient{}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	// Nil payload
	assert.Error(t, sender.SendEmail(context.Background(), nil))

	// Missing From
	err := sender.SendEmail(context.Background(), workerModels.SendEmailPayload{
		To:      []string{"to@example.com"},
		Subject: "Test",
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "email 'from' address is required"))

	// Missing To
	err = sender.SendEmail(context.Background(), workerModels.SendEmailPayload{
		From:    "from@example.com",
		Subject: "Test",
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "email 'to' address is required"))

	// Missing Subject
	err = sender.SendEmail(context.Background(), workerModels.SendEmailPayload{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "email 'subject' is required"))

	// Empty batch
	err = sender.SendEmail(context.Background(), []workerModels.SendEmailPayload{})
	assert.Error(t, err)
}

func TestSendEmail_ClientError(t *testing.T) {
	mockClient := &mockResendClient{
		sendEmailFunc: func(ctx context.Context, req *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
			return nil, errors.New("resend api rate limit")
		},
	}
	cfg := &configs.Config{
		Email: configs.EmailConfig{
			APIKey: "re_test_key",
		},
	}
	sender := NewEmailSenderWithClient(cfg, mockClient)

	err := sender.SendEmail(context.Background(), workerModels.SendEmailPayload{
		From:    "from@example.com",
		To:      []string{"to@example.com"},
		Subject: "Test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resend api rate limit")
}
