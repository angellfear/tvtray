package samsung

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// UPnP RenderingControl on the TV's DMR endpoint (port 9197 on Tizen TVs).
const renderingControl = "urn:schemas-upnp-org:service:RenderingControl:1"

func soap(ctx context.Context, ip, action, args string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	body := `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>` +
		`<u:` + action + ` xmlns:u="` + renderingControl + `"><InstanceID>0</InstanceID>` + args + `</u:` + action + `>` +
		`</s:Body></s:Envelope>`
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+ip+":9197/upnp/control/RenderingControl1", strings.NewReader(body))
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+renderingControl+`#`+action+`"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upnp %s: HTTP %d", action, resp.StatusCode)
	}
	return string(data), nil
}

func xmlValue(doc, tag string) (string, bool) {
	m := regexp.MustCompile(`<` + tag + `>([^<]*)</` + tag + `>`).FindStringSubmatch(doc)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func GetVolume(ctx context.Context, ip string) (int, error) {
	doc, err := soap(ctx, ip, "GetVolume", "<Channel>Master</Channel>")
	if err != nil {
		return 0, err
	}
	v, ok := xmlValue(doc, "CurrentVolume")
	if !ok {
		return 0, fmt.Errorf("upnp GetVolume: no value")
	}
	return strconv.Atoi(v)
}

func SetVolume(ctx context.Context, ip string, v int) error {
	_, err := soap(ctx, ip, "SetVolume", fmt.Sprintf("<Channel>Master</Channel><DesiredVolume>%d</DesiredVolume>", v))
	return err
}

func GetMute(ctx context.Context, ip string) (bool, error) {
	doc, err := soap(ctx, ip, "GetMute", "<Channel>Master</Channel>")
	if err != nil {
		return false, err
	}
	v, _ := xmlValue(doc, "CurrentMute")
	return v == "1", nil
}

func SetMute(ctx context.Context, ip string, mute bool) error {
	v := "0"
	if mute {
		v = "1"
	}
	_, err := soap(ctx, ip, "SetMute", "<Channel>Master</Channel><DesiredMute>"+v+"</DesiredMute>")
	return err
}
