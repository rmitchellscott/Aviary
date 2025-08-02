// Utility functions for handling shared files from Web Share Target

export interface SharedData {
  title: string;
  text: string;
  url: string;
}

export async function getSharedFiles(): Promise<File[]> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('AviarySharedFiles', 1);
    
    request.onerror = () => resolve([]); // Return empty array on error
    
    request.onsuccess = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      
      if (!db.objectStoreNames.contains('files')) {
        resolve([]);
        return;
      }
      
      const transaction = db.transaction(['files'], 'readonly');
      const store = transaction.objectStore('files');
      const getAllRequest = store.getAll();
      
      getAllRequest.onsuccess = () => {
        const storedFiles = getAllRequest.result;
        
        // Convert stored data back to File objects
        const files = storedFiles.map(stored => {
          const blob = new Blob([stored.data], { type: stored.type });
          return new File([blob], stored.name, {
            type: stored.type,
            lastModified: stored.lastModified
          });
        });
        
        resolve(files);
      };
      
      getAllRequest.onerror = () => resolve([]);
    };
  });
}

export async function getSharedData(): Promise<SharedData | null> {
  return new Promise((resolve) => {
    const request = indexedDB.open('AviarySharedData', 1);
    
    request.onerror = () => resolve(null);
    
    request.onsuccess = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      
      if (!db.objectStoreNames.contains('data')) {
        resolve(null);
        return;
      }
      
      const transaction = db.transaction(['data'], 'readonly');
      const store = transaction.objectStore('data');
      const getRequest = store.get('shared');
      
      getRequest.onsuccess = () => {
        const data = getRequest.result;
        resolve(data ? { title: data.title, text: data.text, url: data.url } : null);
      };
      
      getRequest.onerror = () => resolve(null);
    };
  });
}

export async function clearSharedFiles(): Promise<void> {
  return new Promise((resolve) => {
    const request = indexedDB.open('AviarySharedFiles', 1);
    
    request.onsuccess = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      
      if (db.objectStoreNames.contains('files')) {
        const transaction = db.transaction(['files'], 'readwrite');
        const store = transaction.objectStore('files');
        store.clear();
      }
      
      resolve();
    };
    
    request.onerror = () => resolve();
  });
}

export async function clearSharedData(): Promise<void> {
  return new Promise((resolve) => {
    const request = indexedDB.open('AviarySharedData', 1);
    
    request.onsuccess = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      
      if (db.objectStoreNames.contains('data')) {
        const transaction = db.transaction(['data'], 'readwrite');
        const store = transaction.objectStore('data');
        store.clear();
      }
      
      resolve();
    };
    
    request.onerror = () => resolve();
  });
}