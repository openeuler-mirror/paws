import yaml
import os

from utils.utilities import setup_logging

dirname = os.path.dirname(__file__)
LOGGER = setup_logging(__file__)
try:
    LOGGER.info("Reading config from "+dirname)
    config_file = os.path.join(dirname, 'config/recommender_config.yaml')
    config = yaml.load(open(config_file,"r"), Loader=yaml.FullLoader)
except Exception as e:
    LOGGER.error(e)
    config_file = os.path.join(dirname, 'recommender_config.yaml')
    config = yaml.load(open(config_file,"r"), Loader=yaml.FullLoader)


# Retrieve the configuration for the core
RECOMMENDER_NAME = config['RECOMMENDER_NAME']
DEFAULT_NAMESPACE = config['DEFAULT_NAMESPACE']
PROM_URL=config['PROM_URL']
PROM_TOKEN=config['PROM_TOKEN']

# Retrieve the configuration for the recommendation algorithm
HISTORY_LENGTH = config['HISTORY_LENGTH']
FORECASTING_DAYS = config['FORECASTING_DAYS']
SLEEP_WINDOW = config['SLEEP_WINDOW']
STEP_SIZE_DEFAULT = config['STEP_SIZE_DEFAULT']
STEP_SIZE_THROTTLE = config['STEP_SIZE_THROTTLE']

#Others
DEBUG = config['DEBUG']
DEFAULT_VPA_UPDATE_INTERVAL = config['DEFAULT_VPA_UPDATE_INTERVAL']
DEFAULT_DIURNAL_LENGTH = config['DEFAULT_DIURNAL_LENGTH']
