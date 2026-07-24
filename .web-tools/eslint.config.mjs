import js from "@eslint/js";
import globals from "globals";

export default [
  {
    files: ["**/app.js"],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: "script",
      globals: globals.browser,
    },
    rules: js.configs.recommended.rules,
  },
  {
    files: ["**/sw.js"],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: "script",
      globals: globals.serviceworker,
    },
    rules: js.configs.recommended.rules,
  },
];
