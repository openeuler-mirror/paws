# A basic class to store previous VPA recommendation objects.
class VPARecommendations:

    def __init__(self):
        self.vpa_recommendation_objects = {}

    def set_object(self, key, value):
        self.vpa_recommendation_objects[key] = value

    def get_object(self, key):
        return self.vpa_recommendation_objects.get(key, None)
