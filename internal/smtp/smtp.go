package smtp

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"github.com/rmitchellscott/aviary/internal/database"
)

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	UseTLS   bool
}

// EmailData holds data for email templates
type EmailData struct {
	Username    string
	ResetToken  string
	ResetURL    string
	SiteName    string
	SiteURL     string
	ExpiryHours int
}

// GetSMTPConfig reads SMTP configuration from environment variables
func GetSMTPConfig() (*SMTPConfig, error) {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return nil, fmt.Errorf("SMTP_HOST not configured")
	}

	portStr := os.Getenv("SMTP_PORT")
	if portStr == "" {
		portStr = "587"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if from == "" {
		return nil, fmt.Errorf("SMTP_FROM not configured")
	}

	useTLS := true
	if tlsStr := os.Getenv("SMTP_TLS"); tlsStr != "" {
		useTLS = strings.ToLower(tlsStr) == "true"
	}

	return &SMTPConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		UseTLS:   useTLS,
	}, nil
}

// IsSMTPConfigured checks if SMTP is properly configured
func IsSMTPConfigured() bool {
	_, err := GetSMTPConfig()
	return err == nil
}

// SendPasswordResetEmail sends a password reset email
func SendPasswordResetEmail(email, username, resetToken string) error {
	config, err := GetSMTPConfig()
	if err != nil {
		return fmt.Errorf("SMTP not configured: %w", err)
	}

	// Get site URL from environment or use default
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8000"
	}

	// Build reset URL
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", siteURL, resetToken)

	// Get expiry hours from system settings
	expiryStr, _ := database.GetSystemSetting("password_reset_timeout_hours")
	expiryHours := 24 // Default
	if exp, err := strconv.Atoi(expiryStr); err == nil {
		expiryHours = exp
	}

	emailData := EmailData{
		Username:    username,
		ResetToken:  resetToken,
		ResetURL:    resetURL,
		SiteName:    "Aviary",
		SiteURL:     siteURL,
		ExpiryHours: expiryHours,
	}

	// Generate email content
	subject := "Password Reset Request - Aviary"
	htmlBody, err := generatePasswordResetHTML(emailData)
	if err != nil {
		return fmt.Errorf("failed to generate email HTML: %w", err)
	}

	textBody := generatePasswordResetText(emailData)

	// Send email
	return sendEmail(config, email, subject, textBody, htmlBody)
}

// SendWelcomeEmail sends a welcome email to new users
func SendWelcomeEmail(email, username string) error {
	config, err := GetSMTPConfig()
	if err != nil {
		return fmt.Errorf("SMTP not configured: %w", err)
	}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8000"
	}

	emailData := EmailData{
		Username: username,
		SiteName: "Aviary",
		SiteURL:  siteURL,
	}

	subject := "Welcome to Aviary!"
	htmlBody, err := generateWelcomeHTML(emailData)
	if err != nil {
		return fmt.Errorf("failed to generate welcome email HTML: %w", err)
	}

	textBody := generateWelcomeText(emailData)

	return sendEmail(config, email, subject, textBody, htmlBody)
}

// sendEmail sends an email using SMTP
func sendEmail(config *SMTPConfig, to, subject, textBody, htmlBody string) error {
	// Create message
	headers := make(map[string]string)
	headers["From"] = config.From
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary123\""

	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")

	// Add multipart content
	message.WriteString("--boundary123\r\n")
	message.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	message.WriteString("\r\n")
	message.WriteString(textBody)
	message.WriteString("\r\n")

	message.WriteString("--boundary123\r\n")
	message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	message.WriteString("\r\n")
	message.WriteString(htmlBody)
	message.WriteString("\r\n")

	message.WriteString("--boundary123--\r\n")

	// Setup authentication
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// Send email
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	return smtp.SendMail(addr, auth, config.From, []string{to}, message.Bytes())
}

// generatePasswordResetHTML generates HTML content for password reset email
func generatePasswordResetHTML(data EmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Password Reset - {{.SiteName}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f4f4f4; padding: 20px; text-align: center; }
        .content { padding: 20px; }
        .button { display: inline-block; padding: 10px 20px; background: #007cba; color: white; text-decoration: none; border-radius: 5px; }
        .footer { background: #f4f4f4; padding: 20px; text-align: center; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.SiteName}}</h1>
        </div>
        <div class="content">
            <h2>Password Reset Request</h2>
            <p>Hello {{.Username}},</p>
            <p>We received a request to reset your password for your {{.SiteName}} account.</p>
            <p>Click the button below to reset your password:</p>
            <p><a href="{{.ResetURL}}" class="button">Reset Password</a></p>
            <p>If the button doesn't work, copy and paste this link into your browser:</p>
            <p><a href="{{.ResetURL}}">{{.ResetURL}}</a></p>
            <p><strong>Important:</strong> This link will expire in {{.ExpiryHours}} hours.</p>
            <p>If you didn't request this password reset, please ignore this email. Your password will remain unchanged.</p>
        </div>
        <div class="footer">
            <p>This email was sent by {{.SiteName}} (<a href="{{.SiteURL}}">{{.SiteURL}}</a>)</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("password_reset").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generatePasswordResetText generates plain text content for password reset email
func generatePasswordResetText(data EmailData) string {
	return fmt.Sprintf(`Password Reset Request - %s

Hello %s,

We received a request to reset your password for your %s account.

To reset your password, please visit the following link:
%s

This link will expire in %d hours.

If you didn't request this password reset, please ignore this email. Your password will remain unchanged.

--
This email was sent by %s (%s)
`, data.SiteName, data.Username, data.SiteName, data.ResetURL, data.ExpiryHours, data.SiteName, data.SiteURL)
}

// generateWelcomeHTML generates HTML content for welcome email
func generateWelcomeHTML(data EmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to {{.SiteName}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f4f4f4; padding: 20px; text-align: center; }
        .content { padding: 20px; }
        .button { display: inline-block; padding: 10px 20px; background: #007cba; color: white; text-decoration: none; border-radius: 5px; }
        .footer { background: #f4f4f4; padding: 20px; text-align: center; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to {{.SiteName}}!</h1>
        </div>
        <div class="content">
            <h2>Hello {{.Username}},</h2>
            <p>Welcome to {{.SiteName}}! Your account has been created successfully.</p>
            <p>{{.SiteName}} is a webhook-driven document uploader that automatically downloads and sends PDFs to your reMarkable tablet.</p>
            <p>You can now:</p>
            <ul>
                <li>Upload documents from URLs or local files</li>
                <li>Manage your API keys for programmatic access</li>
                <li>Configure your reMarkable settings</li>
                <li>Organize your documents with folders</li>
            </ul>
            <p><a href="{{.SiteURL}}" class="button">Go to {{.SiteName}}</a></p>
        </div>
        <div class="footer">
            <p>This email was sent by {{.SiteName}} (<a href="{{.SiteURL}}">{{.SiteURL}}</a>)</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("welcome").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateWelcomeText generates plain text content for welcome email
func generateWelcomeText(data EmailData) string {
	return fmt.Sprintf(`Welcome to %s!

Hello %s,

Welcome to %s! Your account has been created successfully.

%s is a webhook-driven document uploader that automatically downloads and sends PDFs to your reMarkable tablet.

You can now:
- Upload documents from URLs or local files
- Manage your API keys for programmatic access
- Configure your reMarkable settings
- Organize your documents with folders

Visit %s to get started!

--
This email was sent by %s (%s)
`, data.SiteName, data.Username, data.SiteName, data.SiteName, data.SiteURL, data.SiteName, data.SiteURL)
}

// TestSMTPConnection tests the SMTP connection
func TestSMTPConnection() error {
	config, err := GetSMTPConfig()
	if err != nil {
		return err
	}

	// Setup authentication
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// Test connection
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Quit()

	// Test auth
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	return nil
}