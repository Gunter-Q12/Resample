import numpy as np


class Sine:
    def __init__(self, frequency=1, amplitude=1, theta=0, length=10,
                 quant_bits=None):
        self.frequency = frequency
        self.amplitude = amplitude
        self.theta = theta
        self.length = length
        self.quant_bits = quant_bits

    def sample(self, sample_rate) -> (np.array, np.array):
        time = np.arange(0, self.length * sample_rate + 1)

        sine_wave = (
                self.amplitude *
                np.sin(2 * np.pi * self.frequency * time / sample_rate + self.theta)
        )
        if self.quant_bits is None:
            return sine_wave
        return quantize(sine_wave, self.quant_bits)


def get_time(sample_rate, sample_num):
    return np.linspace(0, (sample_num-1) / sample_rate, sample_num)


def quantize(wave, bits=16):
    wave = (wave + 1) / 2  # values now range from 0 to 1
    step = 1 / (2 ** bits)

    wave = wave / step
    wave -= (2 ** (bits - 1))
    return np.round(wave)


def save(wave, path, bits=None):
    match bits:
        case 16:
            data = wave.astype(np.int16)
        case 32:
            data = wave.astype(np.int32)
        case None:
            data = wave.astype(np.float64)
        case _:
            raise RuntimeError("Only 16 and 32 bit inegers are supported")
    data.tofile(path)
