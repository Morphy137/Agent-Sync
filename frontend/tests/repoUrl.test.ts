import { describe, it, expect } from "vitest";
import { repositoryPageUrl } from "../src/lib/repoUrl";

describe("repositoryPageUrl", () => {
  it("keeps https pages and drops the .git suffix and credentials", () => {
    expect(repositoryPageUrl("https://github.com/o/r.git")).toBe("https://github.com/o/r");
    expect(repositoryPageUrl("https://user:token@github.com/o/r")).toBe("https://github.com/o/r");
    expect(repositoryPageUrl("http://example.com/o/r/")).toBe("http://example.com/o/r");
  });

  it("converts SSH URLs to their https page", () => {
    expect(repositoryPageUrl("git@github.com:o/r.git")).toBe("https://github.com/o/r");
    expect(repositoryPageUrl("ssh://git@gitlab.com/o/r.git")).toBe("https://gitlab.com/o/r");
  });

  it("returns null for anything that is not a web page", () => {
    expect(repositoryPageUrl("file:///C:/repos/r")).toBeNull();
    expect(repositoryPageUrl("javascript:alert(1)")).toBeNull();
    expect(repositoryPageUrl("C:\repos\r")).toBeNull();
    expect(repositoryPageUrl("not a url")).toBeNull();
  });
});
