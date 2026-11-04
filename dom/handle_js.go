package dom

import (
	"bytes"
	"errors"
	"io"
	"math"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// Host is what a host implements to hear from an editor it mounted.
// Each method is called after the change it reports is drawn, at most
// once per event, never per draw. While one runs, the document is
// locked and a call into the handle is refused with ErrReentered: the
// host reads in a method and acts after it returns, or, from Changed,
// returns the command it wants applied. A method must not panic: on
// wasm a panic ends the program and every editor in the page.
type Host interface {
	// Changed reports what a change drew, a gesture's progress among
	// them, such as a vertex following the pointer. A command it
	// returns with ok is applied in the same undo step as the edit,
	// drawn, and reported to Changed in turn, up to Options.MaxFollow
	// times per event. The edit and its follow-ups stand or fall
	// together: when the host asks for one more than that, or a
	// follow-up is refused, all of them and the edit are taken back,
	// the refusal is reported, and then the change that took them back,
	// for which a follow-up is not applied. One returned while a gesture
	// is under way (the editor's State is not Idle) is refused, and
	// nothing taken back, since it would split the action.
	Changed(ch gw.Change) (follow gw.Command, ok bool)
	// Selected reports the selection after an event that changed it,
	// or with anything selected. The slices are copies, valid only for
	// the call: changing them changes nothing in the editor.
	Selected(v []gw.VertexIndex, e []gw.EdgeIndex)
	// Refused reports an edit refused, with the rule it would break.
	Refused(err error)
	// EditingLabel reports that a vertex's or an edge's label (the
	// other is gw.None) is about to be edited in the input, with the
	// text the input starts with: the label, or the value of the
	// attribute the kind edits in its place (gw.VertexKind.EditAttr).
	EditingLabel(v gw.VertexIndex, e gw.EdgeIndex, text string)
}

// NoHost hears nothing. A host embeds it to implement only the methods
// it needs.
type NoHost struct{}

// Changed asks for nothing.
func (NoHost) Changed(gw.Change) (gw.Command, bool) { return gw.Command{}, false }

// Selected does nothing.
func (NoHost) Selected([]gw.VertexIndex, []gw.EdgeIndex) {}

// Refused does nothing.
func (NoHost) Refused(error) {}

// EditingLabel does nothing.
func (NoHost) EditingLabel(gw.VertexIndex, gw.EdgeIndex, string) {}

// DefaultMaxFollow is the most follow-up commands Changed may ask for in
// one event when Options.MaxFollow is 0.
const DefaultMaxFollow = 8

// Options are how an editor is mounted. The zero value is an editor in
// edit mode on an 8-unit grid that patches the drawing by element.
type Options struct {
	View        bool    // mount in view mode
	Grid        float64 // the snap and nudge step; 0 or less for 8
	NoPatch     bool    // redraw everything on every change
	NoExpand    bool    // the overlay does not draw in full the cut texts of the vertex under the pointer
	NoFullText  bool    // a cut text is written without its full text after it (svg.Options.NoFullText)
	IDPrefix    string  // before the renderer's own IDs; give each editor in a page its own
	DocIDPrefix string  // before each vertex's and edge's id in the page
	// MaxFollow is the most follow-up commands Changed may ask for in one
	// event: 0 for DefaultMaxFollow, and at most the document's vertex
	// and edge capacities together, enough for a rule that changes each
	// element once. A rule's commands are undone with the edit, so its
	// longest reaction must also fit within the undo depth.
	MaxFollow int
}

// The refusals of a handle's own calls.
var (
	ErrReentered = errors.New("called from inside the host's own method: return a follow-up from Changed, or call once it returns")
	ErrDestroyed = errors.New("the editor was destroyed")
	errFollow    = errors.New("the host's follow-up commands passed their limit, so the edit was taken back")
	errFollowErr = errors.New("a follow-up command was refused, so the edit was taken back")
	errKept      = errors.New("the edit could not be taken back: it and its follow-ups outgrew the undo log")
	errMaxFollow = errors.New("MaxFollow is less than 0 or more than the document's vertex and edge capacities together")
	errGesture   = errors.New("a follow-up command while a gesture is under way; return it once the editor is idle")
	errMount     = errors.New("mount needs an element, a document and a host")
	errPrefix    = errors.New("an ID prefix may hold only letters, digits, '-' and '_'")
	errZoom      = errors.New("the zoom factor is a positive number")
	errNotes     = errors.New("more annotations than the document has room for")
)

// Handle is one editor mounted in an element.
type Handle struct {
	el        js.Value // the host element
	svgEl     js.Value // the SVG inside it
	palette   js.Value
	edgeItems []js.Value // the palette's edge kind buttons, by kind
	grid      float64    // the grid Mount was given, for each editor Load makes
	noExpand  bool       // no overlay expansion of cut texts, for each editor Load makes
	maxFollow int        // Options.MaxFollow, or its default
	follow    int        // what Options.MaxFollow asked for, checked again against each document Load gives
	input     js.Value
	d         *gw.Document
	ed        *edit.Editor
	r         *svg.Renderer
	lay       *layout.Layouter
	host      Host
	notesV    []svg.Annotation
	notesE    []svg.Annotation
	selV      []gw.VertexIndex // the selection given to the host, which it may change
	selE      []gw.EdgeIndex
	opts      svg.Options
	buf       bytes.Buffer
	funcs     []js.Func
	patch     bool     // patch the DOM by element, else re-render the whole drawing
	lastSel   int      // the selection's size at the last report, to notice a change
	ptr       pointers // the pointer that drives the editor
	inHost    bool     // a Host method is running
	wasLocked bool     // the document was locked before it
	redraw    bool     // a Host method annotated: redraw once the change is reported
	destroyed bool
}

// Mount shows the document in the element, in an editor, and reports
// to the host. The handle holds the document from then on: the host
// reads it through Document and changes it through the handle.
func Mount(el js.Value, d *gw.Document, opts Options, host Host) (*Handle, error) {
	if el.Type() != js.TypeObject || d == nil || host == nil {
		return nil, errMount
	}
	if !token(opts.IDPrefix) || !token(opts.DocIDPrefix) {
		return nil, errPrefix
	}
	maxFollow, err := followFor(opts.MaxFollow, d)
	if err != nil {
		return nil, err
	}
	mode := edit.Edit
	if opts.View {
		mode = edit.View
	}
	h := &Handle{el: el, d: d, r: svg.New(), lay: layout.New(d.Caps()), host: host, patch: !opts.NoPatch, grid: opts.Grid, noExpand: opts.NoExpand, maxFollow: maxFollow, follow: opts.MaxFollow}
	h.opts.IDPrefix, h.opts.DocIDPrefix, h.opts.NoFullText = opts.IDPrefix, opts.DocIDPrefix, opts.NoFullText
	h.ed = edit.New(d, mode, h.grid)
	h.ed.SetExpand(!h.noExpand)
	h.notesV = make([]svg.Annotation, d.Caps().Vertices)
	h.notesE = make([]svg.Annotation, d.Caps().Edges)
	h.selV, h.selE = make([]gw.VertexIndex, 0, d.Caps().Vertices), make([]gw.EdgeIndex, 0, d.Caps().Edges)
	h.mount()
	if d.LiveVertices() > 0 {
		h.fitView()
		h.renderAll()
	}
	return h, nil
}

// refuse reports whether a call on the handle is to be refused, and why.
func (h *Handle) refuse() error {
	switch {
	case h.destroyed:
		return ErrDestroyed
	case h.inHost:
		return ErrReentered
	}
	return nil
}

// Document is the document shown, to read; it is changed through the
// handle, so that the drawing follows.
func (h *Handle) Document() *gw.Document { return h.d }

// Editor is the editor, to read: its selection, state and properties.
func (h *Handle) Editor() *edit.Editor { return h.ed }

// Apply applies a command from the host, drawn and reported to Changed
// as an edit is. The change is valid until the next change, a
// follow-up's included.
func (h *Handle) Apply(c gw.Command) (gw.Change, error) {
	if err := h.refuse(); err != nil {
		return gw.Change{}, err
	}
	h.d.Group()
	ch, err := h.d.Apply(c)
	if back := h.after(ch, nil); err == nil {
		err = back
	}
	return ch, err
}

// Undo undoes the last command, or the action it was part of, and
// reports whether there was one.
func (h *Handle) Undo() bool {
	if h.refuse() != nil {
		return false
	}
	h.d.Group()
	ch, ok := h.d.Undo()
	h.report(ch, nil)
	return ok
}

// Redo applies the last undone command again, or its action.
func (h *Handle) Redo() bool {
	if h.refuse() != nil {
		return false
	}
	h.d.Group()
	ch, ok := h.d.Redo()
	h.report(ch, nil)
	return ok
}

// Load shows another document in place of the one shown, in a new
// editor with the handle's options and mode, its annotations cleared.
func (h *Handle) Load(d *gw.Document) error {
	if err := h.refuse(); err != nil {
		return err
	}
	if d == nil {
		return errMount
	}
	maxFollow, err := followFor(h.follow, d)
	if err != nil {
		return err
	}
	h.d, h.maxFollow = d, maxFollow
	h.ed = edit.New(d, h.ed.Mode(), h.grid)
	h.ed.SetExpand(!h.noExpand)
	h.lay = layout.New(d.Caps())
	if c := d.Caps(); c.Vertices != len(h.notesV) || c.Edges != len(h.notesE) {
		h.notesV, h.notesE = make([]svg.Annotation, c.Vertices), make([]svg.Annotation, c.Edges)
		h.selV, h.selE = make([]gw.VertexIndex, 0, c.Vertices), make([]gw.EdgeIndex, 0, c.Edges)
	}
	clear(h.notesV)
	clear(h.notesE)
	h.fitView()
	h.renderAll()
	return nil
}

// Annotate sets the figures drawn on the vertices and edges, by index,
// replacing the last ones; either may be shorter than the document. It
// may be called from a Host method, since it calls no host code: the
// drawing is then redrawn once the change being reported is done.
func (h *Handle) Annotate(vertices, edges []svg.Annotation) error {
	if h.destroyed {
		return ErrDestroyed
	}
	if len(vertices) > len(h.notesV) || len(edges) > len(h.notesE) {
		return errNotes
	}
	clear(h.notesV)
	clear(h.notesE)
	copy(h.notesV, vertices)
	copy(h.notesE, edges)
	if h.inHost {
		h.redraw = true
		return nil
	}
	h.renderAll()
	return nil
}

// SetMode switches between viewing and editing, ending label editing.
func (h *Handle) SetMode(m edit.Mode) error {
	if err := h.refuse(); err != nil {
		return err
	}
	// SetMode cancels label editing, so the input goes with it: left
	// showing, it would take text the editor no longer wants. As for
	// Escape in the input, the editor hears first, so the blur that
	// hiding causes finds nothing to commit.
	if h.ed.State() == edit.EditingText {
		h.handle(edit.Event{Kind: edit.TextCancel})
		h.hideInput()
	}
	h.ed.SetMode(m)
	h.renderAll()
	return nil
}

// Layout lays the document out in the flow given, as an undoable
// Layout command, and shows the result.
func (h *Handle) Layout(o layout.Options, f gw.Flow) (layout.Result, error) {
	if err := h.refuse(); err != nil {
		return layout.Result{}, err
	}
	return h.relayout(o, f.String())
}

// relayout lays out in the flow named, or the document's own for "".
func (h *Handle) relayout(o layout.Options, flow string) (layout.Result, error) {
	text := ""
	if o.All {
		text = "all"
	}
	if _, err := h.d.Apply(gw.Command{Op: gw.Layout, Text: text, Value: flow}); err != nil {
		return layout.Result{}, err
	}
	res, err := h.lay.Run(h.d, o)
	if err != nil {
		return layout.Result{}, err
	}
	h.fitView()
	h.renderAll()
	return res, nil
}

// fitView sets the viewport to the drawing's bounds when any vertex
// lies outside it, so a layout is never off the screen.
func (h *Handle) fitView() {
	if h.d.LiveVertices() == 0 {
		// Nothing to fit: one unit to a pixel of the drawing, so that a
		// vertex placed in it is drawn at its size.
		w, ht := h.drawing().Get("clientWidth").Float(), h.drawing().Get("clientHeight").Float()
		if w <= 0 || ht <= 0 {
			w, ht = 800, 480
		}
		h.d.View = gw.Viewport{W: w, H: ht}
		return
	}
	b := svg.Bounds(h.d)
	// The bounds carry the renderer's margin; the vertices themselves
	// are what must be in view.
	x0, y0, x1, y1 := b.X+svg.Margin, b.Y+svg.Margin, b.X+b.W-svg.Margin, b.Y+b.H-svg.Margin
	v := h.d.View
	if v.W <= 0 || v.H <= 0 || x0 < v.X || y0 < v.Y || x1 > v.X+v.W || y1 > v.Y+v.H {
		h.d.View = b
	}
}

// Zoom zooms the view about its centre, in by a factor above 1 and out
// by one below.
func (h *Handle) Zoom(f float64) error {
	if err := h.refuse(); err != nil {
		return err
	}
	if !(f > 0) || math.IsInf(f, 0) {
		return errZoom
	}
	h.d.Group()
	h.report(h.ed.Zoom(f), nil)
	return nil
}

// Fit sets the view to the drawing's bounds.
func (h *Handle) Fit() error {
	if err := h.refuse(); err != nil {
		return err
	}
	h.d.Group()
	h.report(h.ed.Fit(), nil)
	return nil
}

// SVG writes the current drawing, a document of its own and so without
// the page's ID prefixes.
func (h *Handle) SVG(w io.Writer) error {
	if h.destroyed {
		return ErrDestroyed
	}
	h.ed.Overlay(&h.opts)
	opts := h.opts
	opts.IDPrefix, opts.DocIDPrefix = "", ""
	return h.r.Render(h.d, opts, w)
}

// Destroy removes the editor from the element and releases every
// function it made; every later call is refused.
func (h *Handle) Destroy() error {
	if err := h.refuse(); err != nil {
		return err
	}
	h.unmount()
	h.destroyed = true
	return nil
}

// handle passes an event from the page to the editor and shows what
// it did, as one undo step with any follow-up. An event the page
// dispatches while a Host method runs, such as the blur a focus there
// causes, is dropped: the document is locked.
func (h *Handle) handle(e edit.Event) {
	if h.inHost || h.destroyed {
		return
	}
	h.d.Group()
	ch, err := h.ed.Handle(e)
	h.report(ch, err)
}

// after shows a change and reports it, applying each follow-up the
// host returns, and ends the undo step the caller began with Group. The
// loop is the trampoline that makes a host's reaction a tail call
// rather than a call back in. A reaction that cannot be completed is
// taken back with the edit, and after returns why.
func (h *Handle) after(ch gw.Change, err error) error {
	if err != nil {
		h.refused(err)
	}
	h.draw(ch)
	var back error
	// bound: maxFollow follow-ups, and one more to refuse
	for i := 0; i <= h.maxFollow; i++ {
		if len(ch.Vertices) == 0 && len(ch.Edges) == 0 && !ch.Layout {
			break
		}
		next, ok := h.changed(ch)
		if !ok {
			break
		}
		if h.ed.State() != edit.Idle {
			h.refused(errGesture)
			break
		}
		if i == h.maxFollow {
			back = h.takeBack(errFollow, nil)
			break
		}
		if ch, err = h.d.Apply(next); err != nil {
			back = h.takeBack(errFollowErr, err)
			break
		}
		h.draw(ch)
	}
	h.d.EndGroup()
	if h.redraw {
		h.redraw = false
		h.renderAll()
	}
	sv, se := h.ed.Selected(h.selV[:0], h.selE[:0])
	if n := len(sv) + len(se); n != h.lastSel || n > 0 {
		h.lastSel = n
		h.enter()
		h.host.Selected(sv, se)
		h.leave()
	}
	return back
}

// report is after for an event of the page's, whose refusals the host
// hears through Refused alone.
func (h *Handle) report(ch gw.Change, err error) { _ = h.after(ch, err) }

// takeBack takes back the edit and the follow-ups applied so far, as the
// host's reaction could not be completed: it reports why, with the
// follow-up's own refusal if there was one, then the change that took
// them back, to Changed, whose follow-up is not applied. It returns the
// refusal reported.
func (h *Handle) takeBack(why, cause error) error {
	ch, ok := h.d.Discard()
	if !ok {
		why = errKept
	}
	if cause != nil {
		why = errors.Join(why, cause)
	}
	h.refused(why)
	if ok {
		h.draw(ch)
		_, _ = h.changed(ch)
	}
	return why
}

// followFor is the follow-up bound for the document: the one asked for,
// or the default, if the document's capacities allow it.
func followFor(asked int, d *gw.Document) (int, error) {
	c := d.Caps()
	if asked < 0 || asked > c.Vertices+c.Edges {
		return 0, errMaxFollow
	}
	if asked == 0 {
		return DefaultMaxFollow, nil
	}
	return asked, nil
}

// draw patches the drawing from a change, or redraws it.
func (h *Handle) draw(ch gw.Change) {
	if ch.Layout || !h.patch {
		h.renderAll()
		return
	}
	h.patchElements(ch)
	h.renderOverlay()
}

// changed calls the host's Changed.
func (h *Handle) changed(ch gw.Change) (gw.Command, bool) {
	h.enter()
	c, ok := h.host.Changed(ch)
	h.leave()
	return c, ok
}

// refused calls the host's Refused.
func (h *Handle) refused(err error) {
	h.enter()
	h.host.Refused(err)
	h.leave()
}

// enter and leave bracket a call into host code: the document is
// locked, as it was or not before, and the handle refuses calls.
func (h *Handle) enter() {
	h.inHost, h.wasLocked = true, h.d.Locked()
	h.d.Lock()
}

func (h *Handle) leave() {
	if !h.wasLocked {
		h.d.Unlock()
	}
	h.inHost = false
}
