import {
  Book,
  Bot,
  Code,
  Folder,
  Puzzle,
  Settings,
  Shield,
  Star,
  Tag,
  Terminal,
} from "@lucide/svelte";
import { t } from "../../i18n";

export const CATEGORY_ICONS = {
  folder: Folder,
  book: Book,
  code: Code,
  terminal: Terminal,
  puzzle: Puzzle,
  bot: Bot,
  shield: Shield,
  star: Star,
  tag: Tag,
  settings: Settings,
} as const;
const BUILTIN_ICONS: Record<string, keyof typeof CATEGORY_ICONS> = {
  skill: "puzzle",
  mcp: "terminal",
  instructions: "book",
};

export function categoryIconId(icon: string | undefined, id = ""): keyof typeof CATEGORY_ICONS {
  return icon && Object.hasOwn(CATEGORY_ICONS, icon)
    ? (icon as keyof typeof CATEGORY_ICONS)
    : (BUILTIN_ICONS[id] ?? "folder");
}

export function categoryIcon(icon: string | undefined, id = "") {
  return CATEGORY_ICONS[categoryIconId(icon, id)];
}

export const CATEGORY_COLORS = [
  "blue",
  "purple",
  "green",
  "amber",
  "teal",
  "rose",
  "gray",
] as const;
const BUILTIN_CATEGORIES = ["skill", "mcp", "instructions", "uncategorized"];

export function categoryLabel(category: { id: string; name: string }): string {
  return !category.name && BUILTIN_CATEGORIES.includes(category.id)
    ? t(`rules.categories.${category.id}`)
    : category.name;
}

export function categoryColor(color: string): string {
  return `var(--category-${(CATEGORY_COLORS as readonly string[]).includes(color) ? color : "gray"})`;
}
