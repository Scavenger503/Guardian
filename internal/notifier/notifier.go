package notifier

import (
	"fmt"
	"net/http"
	"net/url"
)

type Notifier struct {
	token  string
	chatID string
}

func New(token, chatID string) *Notifier {
	return &Notifier{token: token, chatID: chatID}
}

func (n *Notifier) Send(message string) error {
	if n.token == "" || n.chatID == "" {
		return nil
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)
	resp, err := http.PostForm(endpoint, url.Values{
		"chat_id":    {n.chatID},
		"text":       {message},
		"parse_mode": {"Markdown"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (n *Notifier) UpdateFound(containerName, image, oldDigest, newDigest string) {
	msg := fmt.Sprintf(
		"🔄 *Guardian Update Detected*\n\n"+
			"📦 Container: `%s`\n"+
			"🖼️ Image: `%s`\n"+
			"🔁 Old digest: `%s`\n"+
			"✅ New digest: `%s`\n\n"+
			"Pulling and restarting...",
		containerName, image,
		oldDigest[:12], newDigest[:12],
	)
	n.Send(msg)
}

func (n *Notifier) UpdateApplied(containerName, image string) {
	msg := fmt.Sprintf(
		"✅ *Guardian Update Applied*\n\n"+
			"📦 Container: `%s`\n"+
			"🖼️ Image: `%s`\n\n"+
			"Container restarted successfully.",
		containerName, image,
	)
	n.Send(msg)
}

func (n *Notifier) UpdateFailed(containerName, image string, err error) {
	msg := fmt.Sprintf(
		"❌ *Guardian Update Failed*\n\n"+
			"📦 Container: `%s`\n"+
			"🖼️ Image: `%s`\n"+
			"⚠️ Error: `%s`",
		containerName, image, err.Error(),
	)
	n.Send(msg)
}
