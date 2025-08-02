// Service worker to handle share target file sharing
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);
  
  // Check if this is a share target request
  if (url.searchParams.has('share-target') && event.request.method === 'POST') {
    event.respondWith(handleShareTarget(event.request));
  }
});

async function handleShareTarget(request) {
  try {
    const formData = await request.formData();
    const files = formData.getAll('files');
    const title = formData.get('title') || '';
    const text = formData.get('text') || '';
    const url = formData.get('url') || '';
    
    // Store files in IndexedDB temporarily
    if (files.length > 0) {
      await storeSharedFiles(files);
    }
    
    // Store text/URL data if present
    const sharedData = {
      title,
      text,
      url: url || text || title || ''
    };
    
    if (sharedData.url) {
      await storeSharedData(sharedData);
    }
    
    // Redirect to main app
    return Response.redirect('/', 302);
  } catch (error) {
    console.error('Share target error:', error);
    return Response.redirect('/', 302);
  }
}

async function storeSharedFiles(files) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('AviarySharedFiles', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains('files')) {
        db.createObjectStore('files', { keyPath: 'id' });
      }
    };
    
    request.onsuccess = (event) => {
      const db = event.target.result;
      const transaction = db.transaction(['files'], 'readwrite');
      const store = transaction.objectStore('files');
      
      // Clear existing files first
      store.clear();
      
      // Store new files
      const filePromises = files.map(async (file, index) => {
        const arrayBuffer = await file.arrayBuffer();
        return store.put({
          id: index,
          name: file.name,
          type: file.type,
          data: arrayBuffer,
          lastModified: file.lastModified
        });
      });
      
      Promise.all(filePromises)
        .then(() => resolve())
        .catch(reject);
    };
  });
}

async function storeSharedData(data) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('AviarySharedData', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains('data')) {
        db.createObjectStore('data', { keyPath: 'id' });
      }
    };
    
    request.onsuccess = (event) => {
      const db = event.target.result;
      const transaction = db.transaction(['data'], 'readwrite');
      const store = transaction.objectStore('data');
      
      store.clear();
      store.put({ id: 'shared', ...data });
      
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => reject(transaction.error);
    };
  });
}