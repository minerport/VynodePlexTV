import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import "./styles.css";
import "./appearance.css";
import "./layout.css";
import { applyAppearance, readAppearance } from "./Appearance";
applyAppearance(readAppearance());
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
