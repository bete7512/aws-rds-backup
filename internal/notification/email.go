package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/unplank/rds-backup-lambda/internal/backup"
	"github.com/unplank/rds-backup-lambda/internal/config"
)

type EmailParams struct {
	To      string
	Subject string
	Body    string
}

func SendSuccessEmail(cfg *config.Config, result *backup.Result) error {
	body, err := generateEmailContent(successEmailTemplate, *result)
	if err != nil {
		return err
	}

	return sendEmail(EmailParams{
		To:      cfg.AdminEmail,
		Subject: "RDS Backup Successful",
		Body:    body,
	})
}

func SendFailureEmail(cfg *config.Config, result *backup.Result) error {
	body, err := generateEmailContent(failureEmailTemplate, *result)
	if err != nil {
		return err
	}

	return sendEmail(EmailParams{
		To:      cfg.AdminEmail,
		Subject: "RDS Backup Failed",
		Body:    body,
	})
}

func generateEmailContent(templateStr string, data backup.Result) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return body.String(), nil
}

func sendEmail(params EmailParams) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("SOURCE_REGION")),
	}))
	svc := ses.New(sess)

	input := &ses.SendEmailInput{
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(params.Body),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(params.Subject),
			},
		},
		Source: aws.String(fmt.Sprintf("%s <noreply@uniplank.com>", "Uniplank")),
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(params.To), aws.String("vitea.selam@gmail.com")},
		},
	}

	_, err := svc.SendEmail(input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}
