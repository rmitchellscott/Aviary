import { createContext, useContext, useState, ReactNode } from 'react';

interface FolderRefreshContextType {
  refreshTrigger: number;
  triggerRefresh: () => void;
}

export const FolderRefreshContext = createContext<FolderRefreshContextType | null>(null);

export function useFolderRefresh() {
  const context = useContext(FolderRefreshContext);
  if (!context) {
    throw new Error('useFolderRefresh must be used within a FolderRefreshProvider');
  }
  return context;
}

export function FolderRefreshProvider({ children }: { children: ReactNode }) {
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  const triggerRefresh = () => {
    setRefreshTrigger(prev => prev + 1);
  };

  return (
    <FolderRefreshContext.Provider value={{ refreshTrigger, triggerRefresh }}>
      {children}
    </FolderRefreshContext.Provider>
  );
}