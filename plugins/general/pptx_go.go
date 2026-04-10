package general

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// writePptxGo generates a valid PPTX file using pure Go OOXML.
// This is the zero-dependency fallback when Python is unavailable.
// Produces a clean, professional-looking presentation with:
// - Title slide (first slide) and content slides
// - Blue header bar, clean typography
// - Bullet-point body text
func writePptxGo(path string, slides []slideData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	slideCount := len(slides)

	pptxWriteZip(w, "[Content_Types].xml", pptxContentTypesXML(slideCount))
	pptxWriteZip(w, "_rels/.rels", pptxRootRelsXML)
	pptxWriteZip(w, "ppt/presentation.xml", pptxPresentationXML(slideCount))
	pptxWriteZip(w, "ppt/_rels/presentation.xml.rels", pptxPresentationRelsXML(slideCount))
	pptxWriteZip(w, "ppt/slideMasters/slideMaster1.xml", pptxSlideMasterXML)
	pptxWriteZip(w, "ppt/slideMasters/_rels/slideMaster1.xml.rels", pptxSlideMasterRelsXML)
	pptxWriteZip(w, "ppt/slideLayouts/slideLayout1.xml", pptxSlideLayoutXML)
	pptxWriteZip(w, "ppt/slideLayouts/_rels/slideLayout1.xml.rels", pptxSlideLayoutRelsXML)
	pptxWriteZip(w, "ppt/theme/theme1.xml", pptxThemeXML)

	for i, slide := range slides {
		isTitle := (i == 0)
		pptxWriteZip(w, fmt.Sprintf("ppt/slides/slide%d.xml", i+1), pptxSlideXML(slide, isTitle))
		pptxWriteZip(w, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), pptxSlideRelsXML)
	}

	return nil
}

func pptxWriteZip(w *zip.Writer, name, content string) {
	f, err := w.Create(name)
	if err != nil {
		return
	}
	f.Write([]byte(content))
}

func pptxXMLEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func pptxSlideXML(slide slideData, isTitle bool) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld>
<p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr/>
`)

	titleY := "365125"
	titleH := "1325563"
	titleFontSize := "3600"
	titleColor := "FFFFFF"
	if isTitle {
		titleY = "2286000"
		titleH = "1600200"
		titleFontSize = "4400"
		titleColor = "FFFFFF"
	}

	// Blue header bar background
	barH := "1600200"
	barY := "0"
	if isTitle {
		barH = "6858000"
		barY = "0"
	}
	sb.WriteString(fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="100" name="bg"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
  <a:xfrm><a:off x="0" y="%s"/><a:ext cx="12192000" cy="%s"/></a:xfrm>
  <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
  <a:solidFill><a:srgbClr val="2B579A"/></a:solidFill>
  <a:ln><a:noFill/></a:ln>
</p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></p:txBody>
</p:sp>
`, barY, barH))

	// Title text box
	sb.WriteString(fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
<p:spPr>
  <a:xfrm><a:off x="838200" y="%s"/><a:ext cx="10515600" cy="%s"/></a:xfrm>
</p:spPr>
<p:txBody>
  <a:bodyPr anchor="ctr"/>
  <a:lstStyle/>
  <a:p><a:pPr algn="l"/><a:r><a:rPr lang="zh-CN" sz="%s" b="1" dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="Calibri"/><a:ea typeface="Microsoft YaHei"/></a:rPr><a:t>%s</a:t></a:r></a:p>
</p:txBody>
</p:sp>
`, titleY, titleH, titleFontSize, titleColor, pptxXMLEscape(slide.Title)))

	// Body text (only for non-title slides with content)
	if !isTitle && slide.Body != "" {
		lines := strings.Split(slide.Body, "\n")
		sb.WriteString(`<p:sp>
<p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr>
  <a:xfrm><a:off x="838200" y="1825625"/><a:ext cx="10515600" cy="4351338"/></a:xfrm>
</p:spPr>
<p:txBody>
  <a:bodyPr/>
  <a:lstStyle/>
`)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				sb.WriteString(`<a:p><a:endParaRPr lang="zh-CN" sz="1800"/></a:p>`)
				continue
			}
			isBullet := strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")
			text := line
			if isBullet {
				text = strings.TrimSpace(line[2:])
			}
			sb.WriteString(`<a:p>`)
			if isBullet {
				sb.WriteString(`<a:pPr marL="457200" indent="-228600"><a:buChar char="•"/></a:pPr>`)
			}
			sb.WriteString(fmt.Sprintf(`<a:r><a:rPr lang="zh-CN" sz="1800" dirty="0"><a:solidFill><a:srgbClr val="333333"/></a:solidFill><a:latin typeface="Calibri"/><a:ea typeface="Microsoft YaHei"/></a:rPr><a:t>%s</a:t></a:r></a:p>`, pptxXMLEscape(text)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</p:txBody>
</p:sp>
`)
	} else if isTitle && slide.Body != "" {
		// Subtitle for title slide
		sb.WriteString(fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="3" name="Subtitle"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
  <a:xfrm><a:off x="838200" y="4000000"/><a:ext cx="10515600" cy="1200000"/></a:xfrm>
</p:spPr>
<p:txBody>
  <a:bodyPr anchor="t"/>
  <a:lstStyle/>
  <a:p><a:pPr algn="l"/><a:r><a:rPr lang="zh-CN" sz="2000" dirty="0"><a:solidFill><a:srgbClr val="D0D8E8"/></a:solidFill><a:latin typeface="Calibri"/><a:ea typeface="Microsoft YaHei"/></a:rPr><a:t>%s</a:t></a:r></a:p>
</p:txBody>
</p:sp>
`, pptxXMLEscape(slide.Body)))
	}

	sb.WriteString(`</p:spTree>
</p:cSld>
</p:sld>`)
	return sb.String()
}

func pptxContentTypesXML(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`  <Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
`, i))
	}
	sb.WriteString("</Types>")
	return sb.String()
}

func pptxPresentationXML(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
                xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:sldIdLst>
`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`    <p:sldId id="%d" r:id="rId%d"/>
`, 255+i, 100+i))
	}
	sb.WriteString(`  </p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`)
	return sb.String()
}

func pptxPresentationRelsXML(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>
`, 100+i, i))
	}
	sb.WriteString("</Relationships>")
	return sb.String()
}

const pptxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

const pptxSlideRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`

const pptxSlideMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
</p:sldMaster>`

const pptxSlideMasterRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

const pptxSlideLayoutXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             type="blank">
  <p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`

const pptxSlideLayoutRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

const pptxThemeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Yunque">
  <a:themeElements>
    <a:clrScheme name="Yunque Blue">
      <a:dk1><a:srgbClr val="333333"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="1F3864"/></a:dk2>
      <a:lt2><a:srgbClr val="F0F4FA"/></a:lt2>
      <a:accent1><a:srgbClr val="2B579A"/></a:accent1>
      <a:accent2><a:srgbClr val="217346"/></a:accent2>
      <a:accent3><a:srgbClr val="B7472A"/></a:accent3>
      <a:accent4><a:srgbClr val="7C3B99"/></a:accent4>
      <a:accent5><a:srgbClr val="2B7BBA"/></a:accent5>
      <a:accent6><a:srgbClr val="D4A017"/></a:accent6>
      <a:hlink><a:srgbClr val="0563C1"/></a:hlink>
      <a:folHlink><a:srgbClr val="954F72"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="Yunque">
      <a:majorFont><a:latin typeface="Calibri"/><a:ea typeface="Microsoft YaHei"/><a:cs typeface=""/></a:majorFont>
      <a:minorFont><a:latin typeface="Calibri"/><a:ea typeface="Microsoft YaHei"/><a:cs typeface=""/></a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="Office">
      <a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>
      <a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>
      <a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
      <a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`
