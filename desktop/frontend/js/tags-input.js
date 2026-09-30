// A tags field: chips for the tags given, and a box to type more. Enter or
// a comma adds what's typed (so does leaving the box), Backspace in an
// empty box takes the last one off, × takes one off. Suggestions come from
// a shared <datalist id="tag-suggestions">.

const datalist = document.getElementById("tag-suggestions");

// setTagSuggestions offers names while typing a tag anywhere.
export function setTagSuggestions(names) {
  datalist.replaceChildren(...names.map((name) => new Option(name)));
}

// tagsInput turns container into a tags field and returns get() and
// set(names).
export function tagsInput(container, { placeholder = "Add a tag…" } = {}) {
  let tags = [];
  container.classList.add("tags-input");

  const input = document.createElement("input");
  input.type = "text";
  input.setAttribute("list", "tag-suggestions");
  input.placeholder = placeholder;
  input.maxLength = 40;
  input.autocomplete = "off";

  const has = (name) => tags.some((t) => t.toLowerCase() === name.toLowerCase());

  function render() {
    container.replaceChildren(
      ...tags.map((name) => {
        const chip = document.createElement("span");
        chip.className = "tag-chip";
        chip.append(name);
        const remove = document.createElement("button");
        remove.type = "button";
        remove.className = "tag-remove";
        remove.setAttribute("aria-label", `Remove tag ${name}`);
        remove.textContent = "×";
        remove.addEventListener("click", () => {
          tags = tags.filter((t) => t !== name);
          render();
          input.focus();
        });
        chip.append(remove);
        return chip;
      }),
      input,
    );
  }

  // add takes what's typed (one or more, split on commas) as tags.
  function add() {
    for (const part of input.value.split(",")) {
      const name = part.split(/\s+/).filter(Boolean).join(" ");
      if (name && !has(name)) tags.push(name);
    }

    input.value = "";
    render();
    input.focus();
  }

  input.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === ",") {
      // Enter adds a tag rather than submitting the form, unless the box
      // is empty.
      if (input.value.trim() !== "" || event.key === ",") {
        event.preventDefault();
        add();
      }
    } else if (event.key === "Backspace" && input.value === "" && tags.length > 0) {
      tags.pop();
      render();
      input.focus();
    }
  });

  // Picking a suggestion fills the box in one go; take it as a tag.
  input.addEventListener("change", () => {
    if (input.value.trim()) add();
  });

  container.addEventListener("click", (event) => {
    if (event.target === container) input.focus();
  });

  render();
  return {
    // get includes whatever is still typed in the box.
    get() {
      if (input.value.trim()) add();
      return [...tags];
    },
    set(names) {
      tags = [...(names ?? [])];
      input.value = "";
      render();
    },
  };
}

// commentWithTags shows an entry's comment with its tags after it.
export function commentWithTags(entry) {
  const content = document.createElement("div");
  if (entry.comment) content.append(entry.comment);
  if (entry.tags?.length) {
    const tags = document.createElement("div");
    for (const name of entry.tags) {
      const chip = document.createElement("span");
      chip.className = "tag";
      chip.textContent = name;
      tags.append(chip);
    }
    content.append(tags);
  }

  return content;
}
