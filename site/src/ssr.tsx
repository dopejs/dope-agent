import { renderToString } from "react-dom/server";

import { App } from "./App";
import type { SitePayload } from "./types";

export function render(payload: SitePayload): string {
  return renderToString(<App payload={payload} initialLocalePath="en" />);
}
