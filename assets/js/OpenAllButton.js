export default class OpenAllButton extends HTMLButtonElement {
  constructor() {
    super();
  }
  connectedCallback() {
    let el = this.parentElement.nextElementSibling;
    for (let i = 0; el && i < 2; ++i) {
      const links = el.querySelectorAll("a.is-artwork");
      if (links.length == 0) {
        el = el.nextElementSibling;
        continue;
      }
      this.links = new Set(Array.from(links).map((el_a) => el_a.href));
      this.innerText = `Open all ${this.links.size}`;
      this.hidden = false;
      this.addEventListener("click", () => {
        for (const href of this.links) {
          window.open(href);
        }
      });
      break;
    }
  }
}

if (customElements)
  customElements.define("button-open-all", OpenAllButton, {
    extends: "button",
  });
