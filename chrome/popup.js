document.addEventListener('DOMContentLoaded', function() {
  chrome.tabs.query({ active: true, currentWindow: true }, function(tabs) {
    const currentTab = tabs[0];

    chrome.scripting.executeScript({
      target: { tabId: currentTab.id },
      func: extractDataFromPage
    }, (results) => {
      if (chrome.runtime.lastError) {
        console.error('Ошибка:', chrome.runtime.lastError);
        document.getElementById('content').textContent = 'Ошибка загрузки';
        return;
      }
      
      const data = results[0].result;
      if (data) {
        chrome.storage.local.set({ data: data }, function() {
          console.log('Данные сохранены.');
          displayData(data);
        });
      }
    });
  });
});

function extractDataFromPage() {
  const links = Array.from(document.querySelectorAll('a'))
    .filter(a => a.href && a.href.startsWith('http'))
    .map(a => ({ href: a.href, text: a.innerText || a.href }));

  return { links: links.slice(0, 15) }; // Оставляем только 15 ссылок
}

document.getElementById('downloadBtn').addEventListener('click', function() {
  chrome.storage.local.get('data', function(result) {
    if (result.data && result.data.links.length > 0) {
      const linksToDownload = result.data.links.slice(0, 15);
      const fileContent = linksToDownload.map(link => link.href).join('\n');
      const blob = new Blob([fileContent], { type: 'text/plain' });
      const url = URL.createObjectURL(blob);

      const a = document.createElement('a');
      a.href = url;
      a.download = 'links.txt';
      a.click();
      URL.revokeObjectURL(url);
    } else {
      alert('Нет данных для скачивания');
    }
  });
});

function displayData(data) {
  const contentDiv = document.getElementById('contentLink');
  contentDiv.innerHTML = '';

  if (!data.links || data.links.length === 0) {
    contentDiv.textContent = 'Нет найденных ссылок';
    return;
  }

  const ul = document.createElement('ul');
  data.links.forEach((link, index) => {
    const li = document.createElement('li');
    const a = document.createElement('a');
    a.href = link.href;
    a.textContent = `${index + 1}. ${link.text}`;
    a.target = '_blank';
    li.appendChild(a);
    ul.appendChild(li);
  });

  contentDiv.appendChild(ul);
}
