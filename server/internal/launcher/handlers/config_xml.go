package launcher

import (
	"encoding/xml"

	"github.com/gofiber/fiber/v2"
)

const (
	launcherClientID = "652064e22d0ba471c6e7befe6fc91c650519e6f1c"
	launcherVersion  = "201412051458"
)

type launcherConfigXML struct {
	XMLName xml.Name `xml:"LauncherConfig"`

	Language                    string `xml:"Language,attr"`
	LauncherURL                 string `xml:"LauncherUrl,attr"`
	LauncherVersion             string `xml:"LauncherVersion,attr"`
	LauncherDataPathRoot        string `xml:"LauncherDataPathRoot,attr"`
	LauncherHTTPDownloadPath    string `xml:"LauncherHttpDownloadPath,attr"`
	LauncherTorrentDownloadPath string `xml:"LauncherTorrentDownloadPath,attr"`
	LauncherNoticeURL           string `xml:"LauncherNoticeUrl,attr"`
	ClientName                  string `xml:"ClientName,attr"`
	ClientURLRoot               string `xml:"ClientUrlRoot,attr"`
	ClientRoot                  string `xml:"ClientRoot,attr"`
	ClientExePath               string `xml:"ClientExePath,attr"`
	LoginURL                    string `xml:"LoginUrl,attr"`
	VolumeSound                 string `xml:"VolumeSound,attr"`
	VolumeBGM                   string `xml:"VolumeBGM,attr"`
	Development                 string `xml:"Development,attr"`
	VersionType                 string `xml:"VersionType,attr"`
	PB                          string `xml:"PB,attr"`
	UIPB                        string `xml:"UIPB,attr"`
	DEDI                        string `xml:"DEDI,attr"`
	LoginServerIP               string `xml:"LoginServerIP,attr"`
	LoginServerPort             string `xml:"LoginServerPort,attr"`
	CompareDir                  string `xml:"CompareDir,attr"`
	AuthURL                     string `xml:"AuthUrl,attr"`
	SocialURL                   string `xml:"SocialUrl,attr"`
	SteamNoticeURL              string `xml:"SteamNoticeUrl,attr"`
}

func (h *Handler) ConfigXML(c *fiber.Ctx) error {
	base := c.BaseURL()
	document := launcherConfigXML{
		Language:                    "en",
		LauncherURL:                 base + "/LivePatch/Launcher/",
		LauncherVersion:             launcherVersion,
		LauncherDataPathRoot:        `LauncherData\`,
		LauncherHTTPDownloadPath:    `Temp\`,
		LauncherTorrentDownloadPath: `pd\`,
		LauncherNoticeURL:           base + "/fogame/notice",
		ClientName:                  "Fear Online Client",
		ClientURLRoot:               base + "/LivePatch/ClientFear/",
		ClientRoot:                  `FEAR_Online\`,
		ClientExePath:               `FEAR_Online\Engine.exe`,
		LoginURL: base + "/dialog/oauth?response_type=code&state=xyz" +
			"&scope=scope_general,scope_billing&lang=en" +
			"&client_id=" + launcherClientID +
			"&redirect_uri=" + base + "/content_only_launcher&",
		VolumeSound:     "90",
		VolumeBGM:       "90",
		Development:     "1",
		VersionType:     "1",
		PB:              "AE",
		UIPB:            "",
		DEDI:            "1",
		LoginServerIP:   h.config.LoginServerIP,
		LoginServerPort: h.config.LoginServerPort,
		CompareDir:      `Game\Worlds\`,
		AuthURL: base + "/dialog/oauth/authorize?response_type=code" +
			"&client_id=" + launcherClientID +
			"&state=xyz&redirect_url=" + base + "/code2token.php&",
		SocialURL:      base + "/social_connect/steam/connect/callback/redirect",
		SteamNoticeURL: base + "/fogame/steam_notice",
	}

	encoded, err := xml.Marshal(document)
	if err != nil {
		h.logger.Error("failed to marshal launcher config: " + err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, "text/xml; charset=utf-16")
	return c.Send(utf16LEWithBOM(`<?xml version="1.0" encoding="utf-16"?>` + "\r\n" + string(encoded) + "\r\n"))
}
