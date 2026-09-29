package main

import (
	"context"
	"slices"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"
	"github.com/stevendborrelli/function-ordering/input/v1beta1"
	"google.golang.org/protobuf/testing/protocmp"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
)

// These cover a Crossplane that advertises CAPABILITY_DEPENDENCIES. The
// tests in fn_test.go cover every other Crossplane, where the function
// behaves exactly as function-sequencer does.

const depMR = `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr"}}`

func depState(names ...string) *v1.State {
	s := &v1.State{
		Composite: &v1.Resource{Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"}}`)},
		Resources: map[string]*v1.Resource{},
	}
	for _, n := range names {
		s.Resources[n] = &v1.Resource{Resource: resource.MustStructJSON(depMR)}
	}
	return s
}

func depRequest(in *v1beta1.Input, desired, observed []string, carried ...*v1.Dependency) *v1.RunFunctionRequest {
	req := &v1.RunFunctionRequest{
		Meta: &v1.RequestMeta{Capabilities: []v1.Capability{
			v1.Capability_CAPABILITY_CAPABILITIES,
			v1.Capability_CAPABILITY_DEPENDENCIES,
		}},
		Input:    resource.MustStructObject(in),
		Desired:  depState(desired...),
		Observed: depState(observed...),
	}
	if len(carried) > 0 {
		req.Dependencies = &v1.Dependencies{Items: carried}
	}
	return req
}

func dep(r, dependsOn string) *v1.Dependency {
	return &v1.Dependency{Resource: r, DependsOn: &v1.Dependency_ComposedResource{ComposedResource: dependsOn}}
}

func rules(r ...v1beta1.SequencingRule) *v1beta1.Input {
	return &v1beta1.Input{Rules: r}
}

func seq(names ...resource.Name) v1beta1.SequencingRule {
	return v1beta1.SequencingRule{Sequence: names}
}

func TestRunFunctionDeclaresDependencies(t *testing.T) {
	cases := map[string]struct {
		reason      string
		req         *v1.RunFunctionRequest
		wantDeps    []*v1.Dependency
		wantDesired []string
		wantResults int
	}{
		"Chain": {
			reason:      "Each resource should depend on every resource before it, and nothing should be removed from desired state.",
			req:         depRequest(rules(seq("first", "second", "third")), []string{"first", "second", "third"}, nil),
			wantDeps:    []*v1.Dependency{dep("second", "first"), dep("third", "first"), dep("third", "second")},
			wantDesired: []string{"first", "second", "third"},
		},
		"Patterns": {
			reason:      "A pattern should expand to every matching resource, so the dependencies name resources exactly.",
			req:         depRequest(rules(seq("first-.*", "second")), []string{"first-a", "first-b", "second"}, nil),
			wantDeps:    []*v1.Dependency{dep("second", "first-a"), dep("second", "first-b")},
			wantDesired: []string{"first-a", "first-b", "second"},
		},
		"MissingPredecessor": {
			reason:      "Crossplane ignores a dependency on a resource it doesn't know about, so a successor of one that doesn't exist yet should be held back by omission, as function-sequencer does.",
			req:         depRequest(rules(seq("first", "second")), []string{"second"}, nil),
			wantDesired: []string{},
			wantResults: 1,
		},
		"MissingPredecessorAlreadyCreated": {
			reason:      "A successor that already exists should never be removed from desired state, which would delete it.",
			req:         depRequest(rules(seq("first", "second")), []string{"second"}, []string{"second"}),
			wantDesired: []string{"second"},
			wantResults: 1,
		},
		"ObservedOnly": {
			reason:      "A resource the pipeline has dropped but that still exists should still be ordered, so it's deleted before what it depends on.",
			req:         depRequest(rules(seq("first", "second")), []string{"first"}, []string{"first", "second"}),
			wantDeps:    []*v1.Dependency{dep("second", "first")},
			wantDesired: []string{"first"},
		},
		"CarriedForward": {
			reason:      "Dependencies earlier functions declared should be kept, and not declared twice.",
			req:         depRequest(rules(seq("first", "second")), []string{"first", "second"}, nil, dep("second", "first"), dep("other", "first")),
			wantDeps:    []*v1.Dependency{dep("second", "first"), dep("other", "first")},
			wantDesired: []string{"first", "second"},
		},
		"NoUsages": {
			reason: "Crossplane orders deletion from the dependencies, so a plain rule should compose no Usages even with deletion sequencing enabled.",
			req: depRequest(&v1beta1.Input{
				EnableDeletionSequencing: true,
				Rules:                    []v1beta1.SequencingRule{seq("first", "second")},
			}, []string{"first", "second"}, []string{"first", "second"}),
			wantDeps:    []*v1.Dependency{dep("second", "first")},
			wantDesired: []string{"first", "second"},
		},
		"DeleteOnlyKeepsUsages": {
			reason: "A deleteOnly rule orders deletion but not creation, which a dependency can't express, so it should keep composing Usages.",
			req: depRequest(&v1beta1.Input{
				EnableDeletionSequencing: true,
				Rules:                    []v1beta1.SequencingRule{{Sequence: []resource.Name{"first", "second"}, DeleteOnly: true}},
			}, []string{"first", "second"}, []string{"first", "second"}),
			wantDesired: []string{"first", "second", "second-first-usage"},
		},
		"CreateOnlyHoldsBack": {
			reason:      "A createOnly rule orders creation but not deletion, which a dependency can't express, so it should keep holding resources back.",
			req:         depRequest(rules(v1beta1.SequencingRule{Sequence: []resource.Name{"first", "second"}, CreateOnly: true}), []string{"first", "second"}, nil),
			wantDesired: []string{"first"},
			wantResults: 1,
		},
		"ConditionHoldsBack": {
			reason:      "A conditional rule keeps teardown order when its condition turns false, which a dependency can't express, so it should keep holding resources back.",
			req:         depRequest(rules(v1beta1.SequencingRule{Sequence: []resource.Name{"first", "second"}, Condition: "true"}), []string{"first", "second"}, nil),
			wantDesired: []string{"first"},
			wantResults: 1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("%s\nRunFunction(...): %v", tc.reason, err)
			}
			for _, r := range rsp.GetResults() {
				if r.GetSeverity() == v1.Severity_SEVERITY_FATAL {
					t.Fatalf("%s\nRunFunction(...): fatal result: %s", tc.reason, r.GetMessage())
				}
			}

			if diff := cmp.Diff(tc.wantDeps, rsp.GetDependencies().GetItems(), protocmp.Transform()); diff != "" {
				t.Errorf("%s\nRunFunction(...) dependencies: -want, +got:\n%s", tc.reason, diff)
			}

			got := make([]string, 0, len(rsp.GetDesired().GetResources()))
			for n := range rsp.GetDesired().GetResources() {
				got = append(got, n)
			}
			slices.Sort(got)
			if diff := cmp.Diff(tc.wantDesired, got); diff != "" {
				t.Errorf("%s\nRunFunction(...) desired resources: -want, +got:\n%s", tc.reason, diff)
			}

			if n := len(rsp.GetResults()); n != tc.wantResults {
				t.Errorf("%s\nRunFunction(...): want %d results, got %d: %v", tc.reason, tc.wantResults, n, rsp.GetResults())
			}
		})
	}
}
