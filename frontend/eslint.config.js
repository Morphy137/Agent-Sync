import ts from "typescript-eslint";
import svelte from "eslint-plugin-svelte";

export default ts.config(
  ...ts.configs.recommended,
  ...svelte.configs["flat/recommended"],
  {
    rules: {
      "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_" }],
      "@typescript-eslint/no-explicit-any": "off"
    }
  },
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parserOptions: {
        parser: ts.parser
      }
    }
  },
  {
    ignores: ["dist/", "wailsjs/", "node_modules/"]
  }
);
