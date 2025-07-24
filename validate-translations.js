#!/usr/bin/env node

/**
 * Translation Validation Script for Aviary
 * 
 * This script validates that all translation files have the same structure
 * and keys as the English reference file (en.json).
 * 
 * Usage: node validate-translations.js
 */

const fs = require('fs');
const path = require('path');

const LOCALES_DIR = './locales';
const REFERENCE_LOCALE = 'en';

// ANSI color codes for console output
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  white: '\x1b[37m',
  bold: '\x1b[1m'
};

/**
 * Recursively extracts all nested keys from an object
 * @param {Object} obj - The object to extract keys from
 * @param {string} prefix - Current key prefix
 * @returns {Array<string>} Array of dot-notation keys
 */
function getAllKeys(obj, prefix = '') {
  let keys = [];
  for (const key in obj) {
    if (typeof obj[key] === 'object' && obj[key] !== null && !Array.isArray(obj[key])) {
      keys = keys.concat(getAllKeys(obj[key], prefix + key + '.'));
    } else {
      keys.push(prefix + key);
    }
  }
  return keys;
}

/**
 * Loads and parses a JSON locale file
 * @param {string} locale - Locale code (e.g., 'en', 'da', 'de')
 * @returns {Object|null} Parsed JSON object or null if error
 */
function loadLocaleFile(locale) {
  try {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`);
    const content = fs.readFileSync(filePath, 'utf8');
    return JSON.parse(content);
  } catch (error) {
    console.error(`${colors.red}Error loading ${locale}.json: ${error.message}${colors.reset}`);
    return null;
  }
}

/**
 * Gets all available locale files
 * @returns {Array<string>} Array of locale codes
 */
function getAvailableLocales() {
  try {
    const files = fs.readdirSync(LOCALES_DIR);
    return files
      .filter(file => file.endsWith('.json') && !file.includes('.bak'))
      .map(file => file.replace('.json', ''))
      .filter(locale => locale !== REFERENCE_LOCALE);
  } catch (error) {
    console.error(`${colors.red}Error reading locales directory: ${error.message}${colors.reset}`);
    return [];
  }
}

/**
 * Validates a single locale against the reference
 * @param {string} locale - Locale code to validate
 * @param {Array<string>} referenceKeys - Reference keys from English
 * @returns {Object} Validation results
 */
function validateLocale(locale, referenceKeys) {
  const localeData = loadLocaleFile(locale);
  if (!localeData) {
    return { locale, error: 'Failed to load file', coverage: 0 };
  }

  const localeKeys = getAllKeys(localeData);
  const missingKeys = referenceKeys.filter(key => !localeKeys.includes(key));
  const extraKeys = localeKeys.filter(key => !referenceKeys.includes(key));
  const coverage = ((referenceKeys.length - missingKeys.length) / referenceKeys.length * 100);

  return {
    locale,
    displayName: localeData._meta?.displayName || locale,
    totalKeys: localeKeys.length,
    missingKeys,
    extraKeys,
    coverage: parseFloat(coverage.toFixed(1)),
    isComplete: missingKeys.length === 0
  };
}

/**
 * Generates a missing keys template for a locale
 * @param {Array<string>} missingKeys - Array of missing key paths
 * @returns {Object} Nested object structure for missing keys
 */
function generateMissingKeysTemplate(missingKeys) {
  const template = {};
  
  missingKeys.forEach(keyPath => {
    const parts = keyPath.split('.');
    let current = template;
    
    for (let i = 0; i < parts.length - 1; i++) {
      if (!current[parts[i]]) {
        current[parts[i]] = {};
      }
      current = current[parts[i]];
    }
    
    current[parts[parts.length - 1]] = `[TRANSLATE: ${keyPath}]`;
  });
  
  return template;
}

/**
 * Main validation function
 */
function main() {
  console.log(`${colors.bold}${colors.cyan}🌐 Aviary Translation Validator${colors.reset}\n`);

  // Load reference locale
  const referenceData = loadLocaleFile(REFERENCE_LOCALE);
  if (!referenceData) {
    console.error(`${colors.red}Cannot load reference locale (${REFERENCE_LOCALE}.json)${colors.reset}`);
    process.exit(1);
  }

  const referenceKeys = getAllKeys(referenceData);
  console.log(`${colors.blue}📋 Reference (${REFERENCE_LOCALE}): ${referenceKeys.length} keys${colors.reset}\n`);

  // Get all locales to validate
  const locales = getAvailableLocales();
  if (locales.length === 0) {
    console.error(`${colors.red}No locale files found to validate${colors.reset}`);
    process.exit(1);
  }

  // Validate each locale
  const results = [];
  locales.forEach(locale => {
    const result = validateLocale(locale, referenceKeys);
    results.push(result);
  });

  // Sort by coverage (best first)
  results.sort((a, b) => b.coverage - a.coverage);

  // Display results
  console.log(`${colors.bold}📊 Translation Coverage Report${colors.reset}`);
  console.log('='.repeat(60));

  const fullyTranslated = results.filter(r => r.isComplete);
  const partiallyTranslated = results.filter(r => !r.isComplete);

  if (fullyTranslated.length > 0) {
    console.log(`\n${colors.green}✅ Fully Translated (${fullyTranslated.length} languages):${colors.reset}`);
    fullyTranslated.forEach(result => {
      console.log(`   ${colors.green}${result.locale}${colors.reset} (${result.displayName}) - ${result.coverage}%`);
    });
  }

  if (partiallyTranslated.length > 0) {
    console.log(`\n${colors.yellow}⚠️  Needs Translation (${partiallyTranslated.length} languages):${colors.reset}`);
    partiallyTranslated.forEach(result => {
      const coverageColor = result.coverage >= 80 ? colors.yellow : colors.red;
      console.log(`   ${coverageColor}${result.locale}${colors.reset} (${result.displayName}) - ${coverageColor}${result.coverage}%${colors.reset} (${result.missingKeys.length} missing)`);
      
      if (result.missingKeys.length <= 10) {
        console.log(`     Missing: ${result.missingKeys.join(', ')}`);
      } else {
        console.log(`     Missing: ${result.missingKeys.slice(0, 5).join(', ')}, ... and ${result.missingKeys.length - 5} more`);
      }
      
      if (result.extraKeys.length > 0) {
        console.log(`     ${colors.magenta}Extra keys: ${result.extraKeys.join(', ')}${colors.reset}`);
      }
    });
  }

  // Summary statistics
  const avgCoverage = results.reduce((sum, r) => sum + r.coverage, 0) / results.length;
  const totalMissingKeys = results.reduce((sum, r) => sum + r.missingKeys.length, 0);

  console.log(`\n${colors.bold}📈 Summary:${colors.reset}`);
  console.log(`   Average coverage: ${avgCoverage.toFixed(1)}%`);
  console.log(`   Total missing translations: ${totalMissingKeys}`);
  console.log(`   Languages needing work: ${partiallyTranslated.length}/${results.length}`);

  // Generate templates for missing translations
  if (process.argv.includes('--generate-templates')) {
    console.log(`\n${colors.cyan}🔧 Generating translation templates...${colors.reset}`);
    
    partiallyTranslated.forEach(result => {
      if (result.missingKeys.length > 0) {
        const template = generateMissingKeysTemplate(result.missingKeys);
        const templatePath = path.join(LOCALES_DIR, `${result.locale}.missing.json`);
        
        try {
          fs.writeFileSync(templatePath, JSON.stringify(template, null, 2));
          console.log(`   Generated: ${templatePath}`);
        } catch (error) {
          console.error(`   ${colors.red}Failed to generate ${templatePath}: ${error.message}${colors.reset}`);
        }
      }
    });
  }

  // Exit with error code if there are incomplete translations
  if (partiallyTranslated.length > 0) {
    console.log(`\n${colors.yellow}Use --generate-templates flag to create missing key templates${colors.reset}`);
    process.exit(1);
  } else {
    console.log(`\n${colors.green}🎉 All translations are complete!${colors.reset}`);
    process.exit(0);
  }
}

// Run the validator
if (require.main === module) {
  main();
}

module.exports = {
  getAllKeys,
  loadLocaleFile,
  validateLocale,
  generateMissingKeysTemplate
};