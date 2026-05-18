/** @type {import('tailwindcss').Config} */

// F-5009：插件改为“可选加载”。在无 Node 依赖的全新环境重新编译时，
// 即使未安装 @tailwindcss/forms / @tailwindcss/typography 也不会报错
// （仅跳过对应插件，不影响已生成的 public/vendor/tailwind.min.css）。
// 离线重编译步骤见 backend/README.md「Tailwind 重新编译」。
function optionalPlugin(name) {
  try {
    // eslint-disable-next-line global-require, import/no-dynamic-require
    return require(name);
  } catch {
    return null;
  }
}

const plugins = [
  optionalPlugin("@tailwindcss/forms"),
  optionalPlugin("@tailwindcss/typography")
].filter(Boolean);

module.exports = {
  content: ["./public/**/*.{html,js}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        primary: "#0ea5e9",
        secondary: "#10b981",
        "background-light": "#f3f4f6",
        "background-dark": "#0f172a",
        "surface-light": "#ffffff",
        "surface-dark": "#1e293b",
        "text-light": "#1f2937",
        "text-dark": "#e2e8f0",
        "accent-dark": "#38bdf8",
        "accent-light": "#0284c7"
      },
      fontFamily: {
        display: ["Inter", "sans-serif"]
      },
      borderRadius: {
        DEFAULT: "0.75rem"
      }
    }
  },
  plugins
};
