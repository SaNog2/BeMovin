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

// Dialogs: [data-open-dialog="id"] opens one, [data-close-dialog] or a
// click on the backdrop closes it.
document.addEventListener("click", (e) => {
  const opener = e.target.closest("[data-open-dialog]");
  if (opener) {
    document.getElementById(opener.dataset.openDialog)?.showModal();
    return;
  }
  const closer = e.target.closest("[data-close-dialog]");
  if (closer) {
    closer.closest("dialog")?.close();
    return;
  }
  if (e.target instanceof HTMLDialogElement && e.target.open) {
    const r = e.target.getBoundingClientRect();
    const inside = e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom;
    if (!inside) e.target.close();
  }
});

// Ctrl/Cmd+Enter in a comment box posts it.
document.addEventListener("keydown", (e) => {
  if (e.key !== "Enter" || !(e.ctrlKey || e.metaKey)) return;
  const form = e.target.closest(".comment-form");
  if (form && e.target.value.trim()) {
    e.preventDefault();
    form.requestSubmit();
  }
});
