import js from "@eslint/js";
import globals from "globals";

export default [
  {
    files: [".web-tools/**/*.mjs"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: globals.node,
    },
    rules: js.configs.recommended.rules,
  },
  {
    files: [".web-tools/tests/**/*.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        ...globals.node,
        ...globals.browser,
      },
    },
    rules: js.configs.recommended.rules,
  },
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
