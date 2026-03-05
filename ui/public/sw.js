self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "POST" || url.pathname !== "/share-target") {
    return;
  }

  event.respondWith(
    (async () => {
      try {
        const formData = await event.request.formData();

        const files = formData.getAll("files");
        if (files.length > 0) {
          const fileData = await Promise.all(
            files.map(async (file) => ({
              name: file.name,
              type: file.type,
              data: await file.arrayBuffer(),
              lastModified: file.lastModified,
            }))
          );

          const db = await openDB("AviarySharedFiles", "files");
          const tx = db.transaction("files", "readwrite");
          const store = tx.objectStore("files");
          for (const entry of fileData) {
            store.add(entry);
          }
          await txComplete(tx);
          db.close();
        }

        const title = formData.get("title") || "";
        const text = formData.get("text") || "";
        const sharedUrl = formData.get("url") || "";
        if (title || text || sharedUrl) {
          const db = await openDB("AviarySharedData", "data");
          const tx = db.transaction("data", "readwrite");
          const store = tx.objectStore("data");
          store.put({ title, text, url: sharedUrl }, "shared");
          await txComplete(tx);
          db.close();
        }

        return Response.redirect("/?shared=1", 303);
      } catch (err) {
        console.error("[SW] share-target error:", err);
        return Response.redirect("/?shared=1", 303);
      }
    })(),
  );
});

function openDB(name, storeName) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(name, 1);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(storeName)) {
        db.createObjectStore(storeName, { autoIncrement: true });
      }
    };
    request.onsuccess = () => {
      const db = request.result;
      if (db.objectStoreNames.contains(storeName)) {
        resolve(db);
      } else {
        db.close();
        const delReq = indexedDB.deleteDatabase(name);
        delReq.onsuccess = () => {
          const retry = indexedDB.open(name, 1);
          retry.onupgradeneeded = () => {
            retry.result.createObjectStore(storeName, { autoIncrement: true });
          };
          retry.onsuccess = () => resolve(retry.result);
          retry.onerror = () => reject(retry.error);
        };
        delReq.onerror = () => reject(delReq.error);
      }
    };
    request.onerror = () => reject(request.error);
  });
}

function txComplete(tx) {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
}
