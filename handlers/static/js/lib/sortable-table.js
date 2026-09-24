"use strict";

// initSortableTable makes every <th data-sort-key="..."> in table clickable,
// re-ordering <tbody> rows by the data-sort-value attribute on the matching
// <td> in each row. data-sort-type="number" compares numerically; anything
// else compares as a locale-aware string.
export function initSortableTable(table) {
  const headers = table.querySelectorAll("th[data-sort-key]");
  let currentKey = null;
  let ascending = true;

  function sortBy(th) {
    const key = th.getAttribute("data-sort-key");
    const type = th.getAttribute("data-sort-type") || "string";
    ascending = currentKey === key ? !ascending : true;
    currentKey = key;

    headers.forEach((h) => h.removeAttribute("aria-sort"));
    th.setAttribute("aria-sort", ascending ? "ascending" : "descending");

    const tbody = table.querySelector("tbody");
    const cellIndex = Array.from(th.parentElement.children).indexOf(th);
    const rows = Array.from(tbody.querySelectorAll("tr"));

    rows.sort((rowA, rowB) => {
      const rawA = rowA.children[cellIndex]?.getAttribute("data-sort-value");
      const rawB = rowB.children[cellIndex]?.getAttribute("data-sort-value");
      let comparison;
      if (type === "number") {
        comparison = parseFloat(rawA ?? "0") - parseFloat(rawB ?? "0");
      } else {
        comparison = (rawA ?? "").localeCompare(rawB ?? "");
      }
      return ascending ? comparison : -comparison;
    });

    rows.forEach((row) => tbody.appendChild(row));
  }

  headers.forEach((th) => {
    th.setAttribute("role", "button");
    th.setAttribute("tabindex", "0");
    th.addEventListener("click", () => sortBy(th));
    th.addEventListener("keydown", (evt) => {
      if (evt.key === "Enter" || evt.key === " ") {
        evt.preventDefault();
        sortBy(th);
      }
    });
  });
}
