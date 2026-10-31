import { mount } from "svelte";
import "./style.css";
import App from "./App.svelte";
import { installContextMenuGuard } from "./lib/contextMenu";

installContextMenuGuard();

const target = document.getElementById("app");
if (!target) {
  throw new Error("Target container #app not found");
}

const app = mount(App, {
  target,
});

export default app;
