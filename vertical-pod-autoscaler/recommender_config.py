import yaml
import os
from utils.utilities import setup_logging

# Get the directory of the current script
dirname = os.path.dirname(__file__)
# Set up the logger
LOGGER = setup_logging(__file__)

try:
    LOGGER.info("Reading config from " + dirname)
    config_file = os.path.join(dirname, 'config/recommender_config.yaml')
    
    # Use 'with' statement to ensure the file is closed automatically after reading
    with open(config_file, "r") as f:
        # Load the YAML configuration file
        config = yaml.load(f, Loader=yaml.FullLoader)
except Exception as e:
    # Log the error if reading the configuration fails
    LOGGER.error(f"Error reading config from {config_file}: {e}")
    # If the first file read fails, try reading the fallback configuration
    config_file = os.path.join(dirname, 'recommender_config.yaml')
    with open(config_file, "r") as f:
        config = yaml.load(f, Loader=yaml.FullLoader)

# Retrieve the configuration for the core 
RECOMMENDER_NAME = config['RECOMMENDER_NAME']
DEFAULT_NAMESPACE = config['DEFAULT_NAMESPACE']
PROM_URL = config.get('PROM_URL', 'http://localhost')
PROM_TOKEN = config.get('PROM_TOKEN', '')

# Retrieve the configuration for the recommendation algorithm 
HISTORY_LENGTH = config.get('HISTORY_LENGTH', 30)  # Default history length of 30
FORECASTING_DAYS = config.get('FORECASTING_DAYS', 7)  # Default forecasting days of 7
SLEEP_WINDOW = config.get('SLEEP_WINDOW', 10)  # Default sleep window of 10 seconds
STEP_SIZE_DEFAULT = config.get('STEP_SIZE_DEFAULT', 1)
STEP_SIZE_THROTTLE = config.get('STEP_SIZE_THROTTLE', 0.5)

# Other configurations
DEBUG = config.get('DEBUG', False)
DEFAULT_VPA_UPDATE_INTERVAL = config.get('DEFAULT_VPA_UPDATE_INTERVAL', 3600)  # Default update interval of 3600 seconds
DEFAULT_DIURNAL_LENGTH = config.get('DEFAULT_DIURNAL_LENGTH', 24)  # Default diurnal length of 24 hours

LOGGER.info(f"Configuration loaded successfully: {config}")

