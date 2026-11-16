import { renderBook } from "./views/book";
import { renderCreate } from "./views/create";
import { renderMine } from "./views/mine";

const app = document.querySelector<HTMLElement>("#app")!;
const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>(".tab"));

const views: Record<string, (host: HTMLElement) => Promise<void>> = {
  book: renderBook,
  create: renderCreate,
  mine: renderMine,
};

async function show(name: string): Promise<void> {
  tabs.forEach((t) => t.classList.toggle("active", t.dataset.view === name));
  app.innerHTML = `<div class="hint">Загрузка…</div>`;
  try {
    await views[name](app);
  } catch (e) {
    app.innerHTML = `<div class="error">Не удалось загрузить данные: ${
      e instanceof Error ? e.message : String(e)
    }. Запущен ли бэкенд (mise run dev-backend)?</div>`;
  }
}

tabs.forEach((t) => t.addEventListener("click", () => void show(t.dataset.view!)));
void show("book");
