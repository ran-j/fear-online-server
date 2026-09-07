package launcher

import (
	"path"
	"strings"
	"unicode/utf16"

	"go-service-template/internal/assets"

	"github.com/gofiber/fiber/v2"
)

const (
	emptyContentsList  = `<?xml version="1.0" encoding="utf-16"?>` + "\n" + `<FileDataList/>`
	emptyDeleteList    = `<?xml version="1.0" encoding="utf-16"?>` + "\n" + `<DeleteList/>`
	optionPatchListCSV = "MapIndex,FileName,MapRecord,FileSize,FileTime\r\n"
	langBannerCSV      = "[Default]\r\nbannerpath01=\r\n"
)

func (h *Handler) PatchFile(c *fiber.Ctx) error {
	requested := strings.ToLower(path.Base(c.Params("*")))
	switch requested {
	case "config.xml", "config_t.xml":
		return h.ConfigXML(c)
	case "contentslist.xml":
		return sendUTF16XML(c, emptyContentsList)
	case "deletelist.xml":
		return sendUTF16XML(c, emptyDeleteList)
	case "prerequisitelist.xml", "steam_prerequisitelist.xml":
		return h.sendPatchAsset(c, "public/LivePatch/PrerequisiteList.xml", "text/xml; charset=utf-16")
	case "optionpatchlist.csv":
		return c.Type("csv").SendString(optionPatchListCSV)
	case "langbanner.csv":
		return c.Type("txt").SendString(langBannerCSV)
	}
	return h.sendPatchAsset(c, path.Join("public/LivePatch", c.Params("*")), "")
}

func (h *Handler) sendPatchAsset(c *fiber.Ctx, name, contentType string) error {
	data, err := assets.FS.ReadFile(name)
	if err != nil {
		h.logger.Info("launcher patch file not found: " + name)
		return c.SendStatus(fiber.StatusNotFound)
	}
	if contentType != "" {
		c.Set(fiber.HeaderContentType, contentType)
	}
	return c.Send(data)
}

func sendUTF16XML(c *fiber.Ctx, xml string) error {
	c.Set(fiber.HeaderContentType, "text/xml; charset=utf-16")
	return c.Send(utf16LEWithBOM(xml))
}

func utf16LEWithBOM(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	buf := make([]byte, 0, 2+len(encoded)*2)
	buf = append(buf, 0xff, 0xfe)
	for _, unit := range encoded {
		buf = append(buf, byte(unit), byte(unit>>8))
	}
	return buf
}
