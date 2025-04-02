chrome.runtime.onInstalled.addListener(() => {
  console.log("Расширение установлено.");
});

chrome.action.onClicked.addListener((tab) => {
  chrome.scripting.executeScript({
    target: { tabId: tab.id },
    function: extractDataFromPage
  });
});

function extractDataFromPage() {
  const links = Array.from(document.querySelectorAll("a")).map(a => a.href);
  console.log("Извлечённые ссылки:", links);
}