import type { main } from "../../../wailsjs/go/models";
import { api, parseError } from "../api";
import { t } from "../i18n";
import { describeError } from "../i18n/errors";

/** How often the app looks for a new version while it stays open. */
export const UPDATE_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;
const MANUAL_UPDATE_CODE = "update_manual_required";

type ToastFn = (type: "success" | "error" | "info", message: string) => void;

/** App version, install mode and update availability. */
class UpdateState {
  info = $state<main.AppInfo | null>(null);
  status = $state<main.UpdateStatus | null>(null);
  checking = $state(false);
  installing = $state(false);
  private timer: ReturnType<typeof setInterval> | null = null;

  get available(): boolean {
    return Boolean(this.status?.available && this.status.release);
  }

  /** True when updates can be installed from inside the app (installed, not portable). */
  get canAutoInstall(): boolean {
    return Boolean(this.info?.installed && !this.info.portable);
  }

  async loadInfo() {
    try {
      this.info = await api.getAppInfo();
    } catch {
      this.info = null;
    }
  }

  /** Checks GitHub. Manual checks report the result; background checks stay silent on errors. */
  async check(manual: boolean, toast?: ToastFn) {
    if (this.checking) return;
    this.checking = true;
    try {
      this.status = await api.checkForUpdate();
      if (manual && toast) {
        toast(
          this.available ? "info" : "success",
          this.available
            ? t("updates.available_title", { version: this.status?.release?.version ?? "" })
            : t("updates.up_to_date", { version: this.status?.current ?? "" })
        );
      }
    } catch (err) {
      if (manual && toast) toast("error", describeError(err));
    } finally {
      this.checking = false;
    }
  }

  /** Starts the periodic background check (call once at startup). */
  startAutoCheck(enabled: boolean) {
    this.stopAutoCheck();
    if (!enabled) return;
    void this.check(false);
    this.timer = setInterval(() => void this.check(false), UPDATE_CHECK_INTERVAL_MS);
  }

  stopAutoCheck() {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  openReleasePage() {
    const page = this.status?.release?.pageUrl;
    if (page) api.openURL(page);
  }

  /** Installs the update (the app closes and reopens) or opens the download page. */
  async install(toast: ToastFn) {
    if (!this.available || this.installing) return;
    if (!this.canAutoInstall) {
      this.openReleasePage();
      return;
    }
    this.installing = true;
    try {
      toast("info", t("updates.installing"));
      await api.installUpdate();
    } catch (err) {
      if (parseError(err).code === MANUAL_UPDATE_CODE) this.openReleasePage();
      else toast("error", describeError(err));
    } finally {
      this.installing = false;
    }
  }
}

export const updateState = new UpdateState();
