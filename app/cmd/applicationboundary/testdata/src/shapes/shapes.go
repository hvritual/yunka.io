package shapes

type Reader interface{ Read() }
type narrow struct{ target *wide }

func (*narrow) Read() {}

type wide struct{}

func (*wide) Read()   {}
func (*wide) Delete() {}

func Good() Reader            { return &narrow{} }
func Wide() Reader            { return Reader(&wide{}) } // want `AG-TYPE-003.*extra methods.*Delete`
func Concrete() *narrow       { return &narrow{} }       // want `AG-TYPE-002.*exact contract`
func Dynamic(x Reader) Reader { return x }               // want `AG-TYPE-000.*dynamic implementation is unknown`
type embedded struct{ *wide }

func Embedded() Reader { return &embedded{} } // want `AG-TYPE-003.*extra methods.*Delete`
type exported struct{ Target *wide }

func (*exported) Read()      {}
func State() Reader          { return &exported{} } // want `AG-TYPE-004.*exposes field Target`
func Accept(r Reader) Reader { return &narrow{} }
func Wire() {
	_ = Accept(&narrow{})
	_ = Accept(Reader(&wide{})) // want `AG-TYPE-003.*extra methods.*Delete`
}
