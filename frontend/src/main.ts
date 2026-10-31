import { mount } from "svelte";
import "./style.css";
import App from "./App.svelte";
import { installContextMenuGuard } from "./lib/contextMenu";
import { getStoredTheme, initTheme } from "./lib/theme/theme";

installContextMenuGuard();
// Paint with the last used theme until the saved one is loaded from the config.
initTheme(getStoredTheme());

const target = document.getElementById("app");
if (!target) {
  throw new Error("Target container #app not found");
}

const app = mount(App, {
  target,
});

export default app;
