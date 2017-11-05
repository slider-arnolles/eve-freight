package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"

	"golang.org/x/net/context"

	"github.com/antihax/goesi"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gregjones/httpcache"
	httpmemcache "github.com/gregjones/httpcache/memcache"
)

const (
	cookieName = "eve-freight"
)

var regScopes = []string{
	"publicData",
	"characterLocationRead",
	"characterNavigationWrite",
	"characterAssetsRead",
	"characterSkillsRead",
	"characterContractsRead",
	"corporationAssetsRead",
	"corporationMembersRead",
	"corporationStructuresRead",
	"corporationContractsRead",
	"esi-location.read_location.v1",
	"esi-location.read_ship_type.v1",
	"esi-skills.read_skills.v1",
	"esi-wallet.read_character_wallet.v1",
	"esi-search.search_structures.v1",
	"esi-universe.read_structures.v1",
	"esi-corporations.read_corporation_membership.v1",
	"esi-assets.read_assets.v1",
	"esi-corporations.read_structures.v1",
	"esi-location.read_online.v1",
	"esi-contracts.read_character_contracts.v1",
	"esi-characters.read_fatigue.v1",
	"esi-contracts.read_corporation_contracts.v1",
}

// ESIConfig is the configuration of ESI for a particular purpose
type ESIConfig struct {
	ClientID  string `json:"clientid"`
	SecretKey string `json:"secretkey"`
}

// Config is the configuration of this service
type Config struct {
	Registration *ESIConfig `json:"regapp"`
	Auth         *ESIConfig `json:"authapp"`
	URL          string     `json:"url"`
	CookieSecret string     `json:"cookiesecret"`
	DBString     string     `json:"dbstring"`
}

type authContext struct {
	Authenticator *goesi.SSOAuthenticator
	APIContext    context.Context
	Scopes        []string
}

type appContext struct {
	ESI             *goesi.APIClient
	RegAuthContext  *authContext
	AuthAuthContext *authContext
}

var c = &appContext{}
var config = &Config{}
var store *sessions.CookieStore
var dbConn *sqlx.DB

func loadConfig() {
	raw, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(raw, config)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	loadConfig()

	var err error
	dbConn, err = sqlx.Connect("postgres", config.DBString)

	store = sessions.NewCookieStore([]byte(config.CookieSecret))

	// Connect to the memcache server
	cache := memcache.New("localhost:11211")

	// Create a memcached http client for the CCP APIs.
	transport := httpcache.NewTransport(httpmemcache.NewWithClient(cache))
	transport.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	httpClient := &http.Client{Transport: transport}

	c.ESI = goesi.NewAPIClient(httpClient, "Eve Freight")
	c.AuthAuthContext = &authContext{
		Authenticator: goesi.NewSSOAuthenticator(httpClient, config.Auth.ClientID, config.Auth.SecretKey, config.URL, []string{}),
		APIContext:    context.TODO(),
		Scopes:        []string{},
	}
	c.RegAuthContext = &authContext{
		Authenticator: goesi.NewSSOAuthenticator(httpClient, config.Registration.ClientID, config.Registration.SecretKey, config.URL, regScopes),
		APIContext:    context.TODO(),
		Scopes:        regScopes,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/auth/callback", authHandler)
	log.Fatal(http.ListenAndServe(":8000", r))
}

func getSession(r *http.Request) *sessions.Session {
	session, err := store.Get(r, cookieName)
	if err != nil {
		session, _ = store.New(r, cookieName)
	}

	return session
}

func getUser(s *sessions.Session) (int64, bool) {
	field, ok := s.Values["char"]
	if !ok || field == nil {
		return 0, false
	}

	return field.(int64), true
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)

	delete(session.Values, "char")
	session.Values["authtype"] = "login"
	eveSSO(c.AuthAuthContext, w, r, session)
	return
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)

	uid, ok := getUser(session)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusPermanentRedirect)
		return
	}

	log.Printf("uid=%v", uid)

	p := session.Values["character"]
	if p == nil {
		session.Values["authtype"] = "reg"
		eveSSO(c.RegAuthContext, w, r, session)
		return
	}
	/* 	char := p.(*goesi.VerifyResponse)

	   	contracts, _, err := c.ESI.ESI.ContractsApi.GetCharactersCharacterIdContracts(c.RegAuthContext.APIContext, 3, nil)

	   	log.Printf("char=%v", char.CharacterName)
	   	for a, b := range contracts {
	   		log.Printf("a=%v,b=%+v", a, b)
	   	}*/
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "eve-freight")
	if err != nil {
		http.Error(w, "Invalid session", http.StatusInternalServerError)
		return
	}

	at := session.Values["authtype"]
	if at == nil {
		log.Println("Auth response with no session")
		http.Redirect(w, r, "/", http.StatusPermanentRedirect)
		return
	}

	var ac *authContext
	if at.(string) == "reg" {
		ac = c.RegAuthContext
	} else {
		ac = c.AuthAuthContext
	}

	code, err := eveSSOAnswer(ac, w, r, session)
	if err != nil {
		log.Println(err)
		http.Error(w, "Auth Failure", code)
	}
}

func eveSSO(c *authContext, w http.ResponseWriter, r *http.Request, s *sessions.Session) (int, error) {

	// Generate a random state string
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Save the state on the session
	s.Values["state"] = state
	err := s.Save(r, w)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Generate the SSO URL with the state string
	url := c.Authenticator.AuthorizeURL(state, true, c.Scopes)

	// Send the user to the URL
	http.Redirect(w, r, url, 302)
	return http.StatusMovedPermanently, nil
}

func eveSSOAnswer(c *authContext, w http.ResponseWriter, r *http.Request, s *sessions.Session) (int, error) {

	// get our code and state
	code := r.FormValue("code")
	state := r.FormValue("state")

	// Verify the state matches our randomly generated string from earlier.
	if s.Values["state"] != state {
		return http.StatusInternalServerError, errors.New("Invalid State.")
	}

	// Exchange the code for an Access and Refresh token.
	token, err := c.Authenticator.TokenExchange(code)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Obtain a token source (automaticlly pulls refresh as needed)
	tokSrc, err := c.Authenticator.TokenSource(token)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Assign an auth context to the calls
	c.APIContext = context.WithValue(c.APIContext, goesi.ContextOAuth2, tokSrc)

	// Verify the client (returns clientID)
	v, err := c.Authenticator.Verify(tokSrc)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Save the character
	s.Values["char"] = v.CharacterID
	err = s.Save(r, w)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Redirect to the main page.
	http.Redirect(w, r, "/", 302)
	return http.StatusMovedPermanently, nil
}
