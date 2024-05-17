package core

import "io"

type BasePage struct {
	BaseView
	Title      string
	PageSEO    SEO
	HeaderView Header
	BodyView   View
	FooterView Footer
}

type BaseListPage struct {
	BasePage
}

func (v *BasePage) InitContext(s *Site, parentView View) {
	v.BaseView.AddChildViews(&v.PageSEO, &v.HeaderView, v.BodyView, &v.FooterView)
	v.BaseView.InitContext(s, parentView)
}

func (v *BasePage) RenderResponse(writer io.Writer) (err error) {
	return v.Site.RenderView(writer, v, "BasePage.html")
}

type Header struct {
	BaseView
	HeaderNavLinks  []map[string]any
	ThemeSwitchView ThemeSwitch
	MobileNavView   MobileNav
}

func (v *Header) InitContext(s *Site, pv View) {
	v.BaseView.AddChildViews(&v.ThemeSwitchView, &v.MobileNavView)
	v.BaseView.InitContext(s, pv)
	v.MobileNavView.HeaderNavLinks = v.HeaderNavLinks
}

func (v *Header) RenderResponse(writer io.Writer) error {
	return v.Site.RenderView(writer, v, "Header")
}

type MobileNav struct {
	BaseView
	HeaderNavLinks []map[string]any
	ShowNav        bool
}

func (h *MobileNav) RenderResponse(writer io.Writer) error {
	return h.Site.RenderView(writer, h, "MobileNav")
}

type ThemeSwitch struct {
	BaseView
	ThemeName   string
	IsDarkTheme bool
}

func (h *ThemeSwitch) RenderResponse(writer io.Writer) error {
	return h.Site.HtmlTemplate.ExecuteTemplate(writer, "ThemeSwitch", h)
}

type Footer struct {
	BaseView
	ThemeName   string
	IsDarkTheme bool
}

func (v *Footer) RenderResponse(w io.Writer) error {
	return v.Site.RenderView(w, v, "Footer")
}

type SEO struct {
	BaseView
	Title        string
	Description  string
	OgType       string
	OgImages     []string
	TwImage      string
	CanonicalUrl string
}

func (v *SEO) RenderResponse(w io.Writer) error {
	return v.Site.RenderView(w, v, "SEO")
}

/*
func (v *PageSEO) InitContext(s *Site, pv View) {
	smd := v.Site.SiteMetadata
	SiteUrl := GetProp(v.Site.SiteMetadata, "SiteUrl").(string)
	SocialBanner:= GetProp(v.Site.SiteMetadata, "SocialBanner").(string)
	v.CommonSEO.OgImages = []string{
		SiteUrl + SocialBanner,
	}
	v.CommonSEO.TwImage = SiteUrl + SocialBanner
	v.CommonSEO.InitContext(s, pv)
}
*/
