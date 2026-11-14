// Clicking a set (its card or its bracket under the chart) highlights it.
// Delegated on document so it keeps working after htmx swaps the page.
document.addEventListener("click", (e) => {
  const picker = e.target.closest("[data-pick-set]");
  if (!picker) return;
  const session = picker.closest(".session");
  const set = picker.dataset.pickSet;
  session.querySelectorAll("[data-set]").forEach((el) => {
    el.classList.toggle("is-sel", el.dataset.set === set);
  });
});
