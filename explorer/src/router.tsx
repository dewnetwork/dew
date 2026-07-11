import {
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ExplorerShell } from "@/components/explorer-shell";
import { HomePage } from "@/routes/home-page";
import { BlockPage } from "@/routes/block-page";
import { TxPage } from "@/routes/tx-page";
import { AddressPage } from "@/routes/address-page";

const rootRoute = createRootRoute({
  component: ExplorerShell,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

const blockRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/block/$id",
  component: BlockPage,
});

const txRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tx/$hash",
  component: TxPage,
});

const addressRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/address/$addr",
  component: AddressPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  blockRoute,
  txRoute,
  addressRoute,
]);

export const router = createRouter({
  routeTree,
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
