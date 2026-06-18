package config

import "fmt"

// SetCurrentProvider 切换当前渠道
func SetCurrentProvider(providerID string) error {
	mu.Lock()
	defer mu.Unlock()

	// 验证渠道是否存在
	found := false
	for _, p := range current.Providers {
		if p.ID == providerID {
			found = true
			break
		}
	}
	if !found && providerID != "" {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	current.CurrentProvider = providerID
	return savePersistentConfigLocked()
}

// GetCurrentProvider 获取当前渠道
func GetCurrentProvider() *Provider {
	mu.RLock()
	defer mu.RUnlock()

	if current.CurrentProvider == "" {
		return nil
	}

	for _, p := range current.Providers {
		if p.ID == current.CurrentProvider {
			return &p
		}
	}
	return nil
}

// SetCurrentModels 设置当前选中的模型
func SetCurrentModels(textModel, imageModel string) error {
	mu.Lock()
	defer mu.Unlock()

	if textModel != "" {
		current.TextModel = textModel
	}
	if imageModel != "" {
		current.ImageModel = imageModel
	}

	return savePersistentConfigLocked()
}

// AddModelToProvider 添加模型到指定渠道的模型列表
func AddModelToProvider(providerID, modelID string) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range current.Providers {
		if current.Providers[i].ID == providerID {
			provider := current.Providers[i]
			// 检查是否已存在
			modelExists := false
			for _, m := range current.Providers[i].Models {
				if m == modelID {
					modelExists = true
					break
				}
			}
			if !modelExists {
				current.Providers[i].Models = append(current.Providers[i].Models, modelID)
			}

			entryID := ProviderModelEntryID(providerID, modelID)
			for _, m := range current.CustomModels {
				if m.ID == entryID {
					return savePersistentConfigLocked()
				}
			}
			current.CustomModels = append(current.CustomModels, ModelEntry{
				ID:       entryID,
				Model:    modelID,
				Name:     modelID + " [" + provider.Name + "]",
				Type:     InferModelType(modelID),
				Protocol: provider.Protocol,
				Provider: providerID,
			})
			return savePersistentConfigLocked()
		}
	}
	return fmt.Errorf("provider not found: %s", providerID)
}
