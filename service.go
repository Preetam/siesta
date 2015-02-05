package siesta

import (
	"net/http"
	"path"
	"regexp"
	"strings"
)

var services map[string]*Service = make(map[string]*Service)

type Service struct {
	baseURI string

	pre  []contextHandler
	post []contextHandler

	handlers map[*regexp.Regexp]contextHandler

	routes map[string]*node
}

func NewService(baseURI string) *Service {
	if services[baseURI] != nil {
		panic("service already registered")
	}

	return &Service{
		baseURI:  strings.TrimRight(baseURI, "/"),
		handlers: make(map[*regexp.Regexp]contextHandler),
		routes:   map[string]*node{},
	}
}

func addToChain(f interface{}, chain []contextHandler) []contextHandler {
	m := toContextHandler(f)
	return append(chain, m)
}

func (s *Service) AddPre(f interface{}) {
	s.pre = addToChain(f, s.pre)
}

func (s *Service) AddPost(f interface{}) {
	s.post = addToChain(f, s.post)
}

// Service satisfies the http.Handler interface.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.ServeHTTPInContext(NewSiestaContext(), w, r)
}

func (s *Service) ServeHTTPInContext(c Context, w http.ResponseWriter, r *http.Request) {
	quit := false
	for _, m := range s.pre {
		m(c, w, r, func() {
			quit = true
		})

		if quit {
			break
		}
	}

	if !quit {
		r.URL.Path = strings.TrimRight(r.URL.Path, "/")

		var (
			handler contextHandler
			params  routeParams
		)

		// Lookup the tree for this method
		routeNode, ok := s.routes[r.Method]

		if ok {
			handler, params, _ = routeNode.getValue(r.URL.Path)
		}

		if handler == nil {
			http.NotFoundHandler().ServeHTTP(w, r)
		} else {
			r.ParseForm()
			for _, p := range params {
				r.Form.Set(p.Key, p.Value)
			}

			handler(c, w, r, func() {
				quit = true
			})
		}
	}

	for _, m := range s.post {
		m(c, w, r, func() {
			quit = true
		})

		if quit {
			return
		}
	}
}

func (s *Service) Route(verb, uriPath, usage string, f interface{}) {
	handler := toContextHandler(f)

	if n := s.routes[verb]; n == nil {
		s.routes[verb] = &node{}
	}

	s.routes[verb].addRoute(path.Join(s.baseURI, uriPath), handler)
}

func (s *Service) Register() {
	http.Handle(s.baseURI, s)
	http.Handle(s.baseURI+"/", s)
}
