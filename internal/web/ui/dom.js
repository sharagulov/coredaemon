/** @param {string} tag @param {string} [className] @param {Record<string, string>} [attrs] */
export function el(tag, className, attrs = {}) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  for (const [key, val] of Object.entries(attrs)) {
    if (val != null) node.setAttribute(key, val);
  }
  return node;
}

/** @param {string} src @param {string} [wrapClass] */
export function icon(src, wrapClass = "") {
  const wrap = document.createElement("span");
  if (wrapClass) wrap.className = wrapClass;
  const img = document.createElement("img");
  img.src = src;
  img.alt = "";
  wrap.appendChild(img);
  return wrap;
}
