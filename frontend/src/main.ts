import { createRoot } from "octane";
import { App } from "./App.tsrx";
import "./styles/app.css";

createRoot(document.getElementById("app")!).render(App, {});
