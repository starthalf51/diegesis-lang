package mutate

type Mutation struct {
	Op, Target string
	Expect     []string
}
