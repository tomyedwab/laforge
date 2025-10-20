import { useState, useEffect, useRef, useCallback } from 'preact/hooks';

interface AutocompleteOption {
  id: number;
  label: string;
}

interface AutocompleteProps {
  value: number | null;
  onChange: (value: number | null) => void;
  placeholder: string;
  disabled?: boolean;
  loadOptions: (search: string) => Promise<AutocompleteOption[]>;
}

export function Autocomplete({ value, onChange, placeholder, disabled = false, loadOptions }: AutocompleteProps) {
  const [inputValue, setInputValue] = useState('');
  const [options, setOptions] = useState<AutocompleteOption[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedOption, setSelectedOption] = useState<AutocompleteOption | null>(null);
  const [highlightedIndex, setHighlightedIndex] = useState(-1);
  
  const wrapperRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<number | null>(null);

  // Load initial value if provided
  useEffect(() => {
    if (value && !selectedOption) {
      // For edit mode, we need to load the selected option
      // This is a bit tricky since we don't have the label, so we'll show just the ID
      // In a real app, you might want to fetch the full task details
      setInputValue(`Task #${value}`);
    } else if (!value) {
      setInputValue('');
      setSelectedOption(null);
    }
  }, [value]);

  // Handle click outside to close dropdown
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const loadOptionsDebounced = useCallback(async (search: string) => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    if (search.length < 2) {
      setOptions([]);
      setIsOpen(false);
      return;
    }

    setIsLoading(true);
    setError(null);

    debounceRef.current = window.setTimeout(async () => {
      try {
        const results = await loadOptions(search);
        setOptions(results.slice(0, 20)); // Limit to 20 results
        setIsOpen(results.length > 0);
        setHighlightedIndex(-1);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load options');
        setOptions([]);
        setIsOpen(false);
      } finally {
        setIsLoading(false);
      }
    }, 300);
  }, [loadOptions]);

  const handleInputChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    const value = target.value;
    setInputValue(value);
    setSelectedOption(null);
    
    if (value.trim()) {
      loadOptionsDebounced(value.trim());
    } else {
      setOptions([]);
      setIsOpen(false);
      onChange(null);
    }
  };

  const handleOptionSelect = (option: AutocompleteOption) => {
    setSelectedOption(option);
    setInputValue(option.label);
    setIsOpen(false);
    onChange(option.id);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (!isOpen) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setHighlightedIndex(prev => 
          prev < options.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setHighlightedIndex(prev => prev > 0 ? prev - 1 : -1);
        break;
      case 'Enter':
        e.preventDefault();
        if (highlightedIndex >= 0 && options[highlightedIndex]) {
          handleOptionSelect(options[highlightedIndex]);
        }
        break;
      case 'Escape':
        setIsOpen(false);
        setHighlightedIndex(-1);
        break;
    }
  };

  const handleClear = () => {
    setInputValue('');
    setSelectedOption(null);
    setOptions([]);
    setIsOpen(false);
    onChange(null);
    inputRef.current?.focus();
  };

  return (
    <div class="autocomplete-wrapper" ref={wrapperRef}>
      <div class="autocomplete-input-container">
        <input
          ref={inputRef}
          type="text"
          value={inputValue}
          onInput={handleInputChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          class="autocomplete-input"
          autoComplete="off"
        />
        {inputValue && !disabled && (
          <button
            type="button"
            class="autocomplete-clear"
            onClick={handleClear}
            aria-label="Clear selection"
          >
            ×
          </button>
        )}
        {isLoading && (
          <div class="autocomplete-loading">
            <div class="spinner"></div>
          </div>
        )}
      </div>

      {isOpen && (
        <div class="autocomplete-dropdown">
          {error ? (
            <div class="autocomplete-error">
              {error}
              <button 
                type="button" 
                class="autocomplete-retry"
                onClick={() => loadOptionsDebounced(inputValue)}
              >
                Retry
              </button>
            </div>
          ) : options.length === 0 ? (
            <div class="autocomplete-no-results">
              No results found for "{inputValue}"
            </div>
          ) : (
            <ul class="autocomplete-options">
              {options.map((option, index) => (
                <li
                  key={option.id}
                  class={`autocomplete-option ${
                    index === highlightedIndex ? 'highlighted' : ''
                  } ${
                    selectedOption?.id === option.id ? 'selected' : ''
                  }`}
                  onClick={() => handleOptionSelect(option)}
                  onMouseEnter={() => setHighlightedIndex(index)}
                >
                  {option.label}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}