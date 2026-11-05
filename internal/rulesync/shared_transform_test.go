package rulesync_test

import (
	"path/filepath"
	"testing"

	"agentsync/internal/rules"
	"agentsync/internal/transform"
)

type realTransformer struct{}

func (realTransformer) Render(s rules.Transform, src, dst []byte, previous []string) ([]byte, []string, []byte, error) {
	result, err := transform.Render(transform.Spec{Type: s.Type, Key: s.Key}, src, dst, previous)
	return result.Output, result.Managed, result.ManagedView, err
}
func (realTransformer) SourceView(s rules.Transform, src []byte) ([]byte, error) {
	return transform.SourceView(transform.Spec{Type: s.Type, Key: s.Key}, src)
}
func (realTransformer) TargetView(s rules.Transform, dst []byte, names []string) ([]byte, error) {
	return transform.TargetView(transform.Spec{Type: s.Type, Key: s.Key}, dst, names)
}
func TestSharedCodexMCPRules(t *testing.T) {
	e := newEnv(t)
	e.engine.Transformer = realTransformer{}
	first := filepath.Join(e.repo, "blender.json")
	e.write(first, `{"mcpServers":{"blender":{"command":"blender"}}}`)
	e.write(filepath.Join(e.repo, "blockbench.json"), `{"mcpServers":{"blockbench":{"command":"blockbench"}}}`)
	d := dest("d", aliasA, "config.toml", rules.ModeOneWay)
	d.Transform = &rules.Transform{Type: rules.TransformTOMLMerge, Key: "mcp_servers"}
	all := []rules.Rule{repoRule("blender", "blender.json", d), repoRule("blockbench", "blockbench.json", d)}
	if result := e.sync(all); result.Written != 2 || result.Skipped != 0 {
		t.Fatalf("sync: %+v", result)
	}
	target := filepath.Join(e.locA, "config.toml")
	check := func(expected string) {
		t.Helper()
		view, err := transform.TargetView(transform.Spec{Type: transform.TypeTOMLMerge, Key: "mcp_servers"}, []byte(e.read(target)), []string{"blender", "blockbench"})
		if err != nil || string(view) != expected {
			t.Fatalf("entries: %s, %v", view, err)
		}
	}
	check(`{"blender":{"command":"blender"},"blockbench":{"command":"blockbench"}}`)
	e.write(first, `{"mcpServers":{}}`)
	e.sync(all)
	check(`{"blockbench":{"command":"blockbench"}}`)
}

func TestSharedMCPTransfersPreviouslyManagedEntry(t *testing.T) {
	for _, transformType := range []string{rules.TransformJSONMerge, rules.TransformTOMLMerge} {
		t.Run(transformType, func(t *testing.T) {
			e := newEnv(t)
			e.engine.Transformer = realTransformer{}
			first := filepath.Join(e.repo, "first.json")
			second := filepath.Join(e.repo, "second.json")
			e.write(first, `{"mcpServers":{"blender":{"command":"blender"},"blockbench":{"command":"blockbench"}}}`)
			e.write(second, `{"mcpServers":{"blockbench":{"command":"blockbench"}}}`)
			key, target := "mcpServers", "agents.json"
			if transformType == rules.TransformTOMLMerge {
				key, target = "mcp_servers", "config.toml"
			}
			d := dest("d", aliasA, target, rules.ModeOneWay)
			d.Transform = &rules.Transform{Type: transformType, Key: key}
			owner := repoRule("first", "first.json", d)
			e.sync([]rules.Rule{owner})
			e.write(first, `{"mcpServers":{"blender":{"command":"blender"}}}`)
			all := []rules.Rule{owner, repoRule("second", "second.json", d)}
			if report := e.engine.Status(all); len(report.Collisions) != 0 {
				t.Fatalf("obsolete ownership blocks transfer: %+v", report.Collisions)
			}
			if result := e.sync(all); result.Skipped != 0 {
				t.Fatalf("transfer skipped: %+v", result)
			}
			e.write(second, `{"mcpServers":{"blockbench":{"command":"updated"}}}`)
			if result := e.sync(all); result.Skipped != 0 {
				t.Fatalf("updated entry caused drift: %+v", result)
			}
			view, err := transform.TargetView(transform.Spec{Type: transformType, Key: key}, []byte(e.read(filepath.Join(e.locA, target))), []string{"blender", "blockbench"})
			if err != nil || string(view) != `{"blender":{"command":"blender"},"blockbench":{"command":"updated"}}` {
				t.Fatalf("entries after transfer: %s, %v", view, err)
			}
		})
	}
}
